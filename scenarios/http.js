import datagen from 'k6/x/frostfs/datagen';
import registry from 'k6/x/frostfs/registry';
import http from 'k6/http';
import { SharedArray } from 'k6/data';
import { sleep } from 'k6';

const obj_list = new SharedArray('obj_list', function () {
    return JSON.parse(open(__ENV.PREGEN_JSON)).objects;
});

const container_list = new SharedArray('container_list', function () {
    return JSON.parse(open(__ENV.PREGEN_JSON)).containers;
});

const read_size = JSON.parse(open(__ENV.PREGEN_JSON)).obj_size;

// Select random HTTP endpoint for current VU
const http_endpoints = __ENV.HTTP_ENDPOINTS.split(',');
const http_endpoint = http_endpoints[Math.floor(Math.random() * http_endpoints.length)];
const log = logging.new().withField("endpoint", http_endpoint);

const registry_enabled = !!__ENV.REGISTRY_FILE;
const obj_registry = registry_enabled ? registry.open(__ENV.REGISTRY_FILE) : undefined;

const duration = __ENV.DURATION;

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
    }
}

const read_vu_count = parseInt(__ENV.READERS || '0');
if (read_vu_count > 0) {
    scenarios.read = {
        executor: 'constant-vus',
        vus: read_vu_count,
        duration: `${duration}s`,
        exec: 'obj_read', 
        gracefulStop: '5s',
    }
}

export const options = {
    scenarios,
    setupTimeout: '5s',
};

export function setup() {
    const total_vu_count = write_vu_count + read_vu_count;

    console.log(`Pregenerated containers:       ${container_list.length}`);
    console.log(`Pregenerated read object size: ${read_size}`);
    console.log(`Pregenerated total objects:    ${obj_list.length}`);
    console.log(`Reading VUs:                   ${read_vu_count}`);
    console.log(`Writing VUs:                   ${write_vu_count}`);
    console.log(`Total VUs:                     ${total_vu_count}`);
}

export function teardown(data) {
    if (obj_registry) {
        obj_registry.close();
    }
}

export function obj_write() {
    if (__ENV.SLEEP_WRITE) {
        sleep(__ENV.SLEEP_WRITE);
    }

    const container = container_list[Math.floor(Math.random() * container_list.length)];

    const { payload, hash } = generator.genPayload(registry_enabled);
    const data = {
        field: uuidv4(),
        file: http.file(payload, "random.data"),
    };

    const resp = http.post(`http://${http_endpoint}/upload/${container}`, data);
    if (resp.status != 200) {
        log.withFields({status: resp.status, cid: container}).error(resp.error);
        return;
    }
    const object_id = JSON.parse(resp.body).object_id;
    if (obj_registry) {
        obj_registry.addObject(container, object_id, "", "", hash);
    }
}

export function obj_read() {
    if (__ENV.SLEEP_READ) {
        sleep(__ENV.SLEEP_READ);
    }

    const obj = obj_list[Math.floor(Math.random() * obj_list.length)];
    const resp = http.get(`http://${http_endpoint}/get/${obj.container}/${obj.object}`);
    if (resp.status != 200) {
        log.withFields({status: resp.status, cid: obj.container, oid: obj.object}).error(resp.error);
    }
}

export function uuidv4() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        let r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}
