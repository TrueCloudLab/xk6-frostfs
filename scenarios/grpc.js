import datagen from 'k6/x/frostfs/datagen';
import native from 'k6/x/frostfs/native';
import logging from 'k6/x/frostfs/logging';
import registry from 'k6/x/frostfs/registry';
import { SharedArray } from 'k6/data';
import { sleep } from 'k6';

const obj_list = new SharedArray('obj_list', function () {
    return JSON.parse(open(__ENV.PREGEN_JSON)).objects;
});

const container_list = new SharedArray('container_list', function () {
    return JSON.parse(open(__ENV.PREGEN_JSON)).containers;
});

const read_size = JSON.parse(open(__ENV.PREGEN_JSON)).obj_size;

// Select random gRPC endpoint for current VU
const grpc_endpoints = __ENV.GRPC_ENDPOINTS.split(',');
const grpc_endpoint = grpc_endpoints[Math.floor(Math.random() * grpc_endpoints.length)];
const grpc_client = native.connect(grpc_endpoint, '', __ENV.DIAL_TIMEOUT ? parseInt(__ENV.DIAL_TIMEOUT) : 5, __ENV.STREAM_TIMEOUT ? parseInt(__ENV.STREAM_TIMEOUT) : 15);
const log = logging.new().withField("endpoint", grpc_endpoint);

const write_vu_count = parseInt(__ENV.WRITERS || '0');
const read_vu_count = parseInt(__ENV.READERS || '0');
const delete_vu_count = parseInt(__ENV.DELETERS || '0');
const registry_enabled = !!__ENV.REGISTRY_FILE;
const obj_registry = registry_enabled ? registry.open(__ENV.REGISTRY_FILE, write_vu_count + delete_vu_count) : undefined;

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

if (write_vu_count > 0) {
    scenarios.write = {
        executor: 'constant-vus',
        vus: write_vu_count,
        duration: `${duration}s`,
        exec: 'obj_write', 
        gracefulStop: '5s',
    };
}

if (read_vu_count > 0) {
    scenarios.read = {
        executor: 'constant-vus',
        vus: read_vu_count,
        duration: `${duration}s`,
        exec: 'obj_read', 
        gracefulStop: '5s',
    };
}

if (delete_vu_count > 0) {
    if (!obj_to_delete_selector) {
        throw new Error('Positive DELETE worker number without a proper object selector');
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

    console.log(`Pregenerated containers:       ${container_list.length}`);
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

export function obj_write() {
    if (__ENV.SLEEP_WRITE) {
        sleep(__ENV.SLEEP_WRITE);
    }

    const headers = {
        unique_header: uuidv4()
    };
    const container = container_list[Math.floor(Math.random() * container_list.length)];

    const { payload, hash } = generator.genPayload(registry_enabled);
    const resp = grpc_client.put(container, headers, payload);
    if (!resp.success) {
        log.withField("cid", container).error(resp.error);
        return;
    }

    if (obj_registry) {
        obj_registry.addObject(container, resp.object_id, "", "", hash);
    }
}

export function obj_read() {
    if (__ENV.SLEEP_READ) {
        sleep(__ENV.SLEEP_READ);
    }

    const obj = obj_list[Math.floor(Math.random() * obj_list.length)];
    const resp = grpc_client.get(obj.container, obj.object)
    if (!resp.success) {
        log.withFields({cid: obj.container, oid: obj.object}).error(resp.error);
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

    const resp = grpc_client.delete(obj.c_id, obj.o_id);
    if (!resp.success) {
        // Log errors except (2052 - object already deleted)
        log.withFields({cid: obj.c_id, oid: obj.o_id}).error(resp.error);
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
