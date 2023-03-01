import datagen from 'k6/x/frostfs/datagen';
import registry from 'k6/x/frostfs/registry';
import s3 from 'k6/x/frostfs/s3';
import { SharedArray } from 'k6/data';
import { sleep } from 'k6';
import { textSummary } from './libs/k6-summary-0.0.2.js';

const obj_list = new SharedArray('obj_list', function () {
    return JSON.parse(open(__ENV.PREGEN_JSON)).objects;
});

const bucket_list = new SharedArray('bucket_list', function () {
    return JSON.parse(open(__ENV.PREGEN_JSON)).buckets;
});

const read_size = JSON.parse(open(__ENV.PREGEN_JSON)).obj_size;
const summary_json = __ENV.SUMMARY_JSON || "/tmp/summary.json";

// Select random S3 endpoint for current VU
const s3_endpoints = __ENV.S3_ENDPOINTS.split(',');
const s3_endpoint = s3_endpoints[Math.floor(Math.random() * s3_endpoints.length)];
const s3_client = s3.connect(`http://${s3_endpoint}`);
const log = logging.new().withField("endpoint", s3_endpoint);

const registry_enabled = !!__ENV.REGISTRY_FILE;
const obj_registry = registry_enabled ? registry.open(__ENV.REGISTRY_FILE) : undefined;

const duration = __ENV.DURATION;

const delete_age = __ENV.DELETE_AGE ? parseInt(__ENV.DELETE_AGE) : undefined;
let obj_to_delete_selector = undefined;
if (registry_enabled && delete_age) {
    obj_to_delete_selector = registry.getSelector(
        __ENV.REGISTRY_FILE,
        "obj_to_delete",
        __ENV.SELECTION_SIZE ? parseInt(__ENV.SELECTION_SIZE) : 0,
        {
            status: "created",
            age: delete_age,
        }
    );
}

const generator = datagen.generator(1024 * parseInt(__ENV.WRITE_OBJ_SIZE));

const scenarios = {};

const write_vu_count = parseInt(__ENV.WRITERS || '0');
if (write_vu_count > 0) {
    scenarios.write = {
        executor: 'constant-vus',
        vus: write_vu_count,
        duration: `${duration}s`,
        exec: 'obj_write', 
        gracefulStop: '5s',
    };
}

const read_vu_count = parseInt(__ENV.READERS || '0');
if (read_vu_count > 0) {
    scenarios.read = {
        executor: 'constant-vus',
        vus: read_vu_count,
        duration: `${duration}s`,
        exec: 'obj_read', 
        gracefulStop: '5s',
    };
}

const delete_vu_count = parseInt(__ENV.DELETERS || '0');
if (delete_vu_count > 0) {
    if (!obj_to_delete_selector) {
        throw 'Positive DELETE worker number without a proper object selector';
    }

    scenarios.delete = {
        executor: 'constant-vus',
        vus: delete_vu_count,
        duration: `${duration}s`,
        exec: 'obj_delete', 
        gracefulStop: '5s',
    };
}

export const options = {
    scenarios,
    setupTimeout: '5s',
};

export function setup() {
    const total_vu_count = write_vu_count + read_vu_count + delete_vu_count;

    console.log(`Pregenerated buckets:          ${bucket_list.length}`);
    console.log(`Pregenerated read object size: ${read_size}`);
    console.log(`Pregenerated total objects:    ${obj_list.length}`);
    console.log(`Reading VUs:                   ${read_vu_count}`);
    console.log(`Writing VUs:                   ${write_vu_count}`);
    console.log(`Deleting VUs:                  ${delete_vu_count}`);
    console.log(`Total VUs:                     ${total_vu_count}`);
}

export function teardown(data) {
    if (obj_registry) {
        obj_registry.close();
    }
}

export function handleSummary(data) {
    return {
        'stdout': textSummary(data, { indent: ' ', enableColors: false }),
        [summary_json]: JSON.stringify(data),
    };
  }

export function obj_write() {
    if (__ENV.SLEEP_WRITE) {
        sleep(__ENV.SLEEP_WRITE);
    }

    const key = __ENV.OBJ_NAME || uuidv4();
    const bucket = bucket_list[Math.floor(Math.random() * bucket_list.length)];

    const { payload, hash } = generator.genPayload(registry_enabled);
    const resp = s3_client.put(bucket, key, payload);
    if (!resp.success) {
        log.withFields({bucket: bucket, key: key}).error(resp.error);
        return;
    }

    if (obj_registry) {
        obj_registry.addObject("", "", bucket, key, hash);
    }
}

export function obj_read() {
    if (__ENV.SLEEP_READ) {
        sleep(__ENV.SLEEP_READ);
    }

    const obj = obj_list[Math.floor(Math.random() * obj_list.length)];

    const resp = s3_client.get(obj.bucket, obj.object);
    if (!resp.success) {
        log.withFields({bucket: obj.bucket, key: obj.object}).error(resp.error);
    } 
}

export function obj_delete() {
    if (__ENV.SLEEP_DELETE) {
        sleep(__ENV.SLEEP_DELETE);
    }

    const obj = obj_to_delete_selector.nextObject();
    if (!obj) {
        return;
    }

    const resp = s3_client.delete(obj.s3_bucket, obj.s3_key);
    if (!resp.success) {
        log.withFields({bucket: obj.s3_bucket, key: obj.s3_key, op: "DELETE"}).error(resp.error);
        return;
    }

    obj_registry.deleteObject(obj.id);
}

export function uuidv4() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        let r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}
