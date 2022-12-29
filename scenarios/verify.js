import native from 'k6/x/frostfs/native';
import registry from 'k6/x/frostfs/registry';
import s3 from 'k6/x/frostfs/s3';
import { sleep } from 'k6';
import { Counter } from 'k6/metrics';

const obj_registry = registry.open(__ENV.REGISTRY_FILE);

// Time limit (in seconds) for the run
const time_limit = __ENV.TIME_LIMIT || "60";

// Number of objects in each status. These counters are cumulative in a
// sense that they reflect total number of objects in the registry, not just
// number of objects that were processed by specific run of this scenario.
// This allows to run this scenario multiple times and collect overall
// statistics in the final run.
const obj_counters = {
    verified: new Counter('verified_obj'),
    skipped: new Counter('skipped_obj'),
    invalid: new Counter('invalid_obj'),
};

// Connect to random gRPC endpoint
let grpc_client = undefined;
if (__ENV.GRPC_ENDPOINTS) {
    const grpcEndpoints = __ENV.GRPC_ENDPOINTS.split(',');
    const grpcEndpoint = grpcEndpoints[Math.floor(Math.random() * grpcEndpoints.length)];
    grpc_client = native.connect(grpcEndpoint, '');
}

// Connect to random S3 endpoint
let s3_client = undefined;
if (__ENV.S3_ENDPOINTS) {
    const s3_endpoints = __ENV.S3_ENDPOINTS.split(',');
    const s3_endpoint = s3_endpoints[Math.floor(Math.random() * s3_endpoints.length)];
    s3_client = s3.connect(`http://${s3_endpoint}`);
}

// We will attempt to verify every object in "created" status. The scenario will execute
// as many iterations as there are objects. Each object will have 3 retries to be verified
const obj_to_verify_selector = registry.getSelector(
    __ENV.REGISTRY_FILE,
    "obj_to_verify",
    __ENV.SELECTION_SIZE ? parseInt(__ENV.SELECTION_SIZE) : 0,
    {
        status: "created",
    }
);
const obj_to_verify_count = obj_to_verify_selector.count();
// Execute at least one iteration (executor shared-iterations can't run 0 iterations)
const iterations = Math.max(1, obj_to_verify_count);
// Executor shared-iterations requires number of iterations to be larger than number of VUs
const vus = Math.min(__ENV.CLIENTS, iterations);

const scenarios = {
    verify: {
        executor: 'shared-iterations',
        vus,
        iterations,
        maxDuration: `${time_limit}s`,
        exec: 'obj_verify',
        gracefulStop: '5s',
    }
};

export const options = {
    scenarios,
    setupTimeout: '5s',
};

export function setup() {
    // Populate counters with initial values
    for (const [status, counter] of Object.entries(obj_counters)) {
        const obj_selector = registry.getSelector(
            __ENV.REGISTRY_FILE,
            status,
            __ENV.SELECTION_SIZE ? parseInt(__ENV.SELECTION_SIZE) : 0,
            { status });
        counter.add(obj_selector.count());
    }
}

export function obj_verify() {
    if (__ENV.SLEEP) {
        sleep(__ENV.SLEEP);
    }

    const obj = obj_to_verify_selector.nextObject();
    if (!obj) {
        console.log("All objects have been verified");
        return;
    }

    const obj_status = verify_object_with_retries(obj, 3);
    obj_counters[obj_status].add(1);
    obj_registry.setObjectStatus(obj.id, obj_status);
}

function verify_object_with_retries(obj, attempts) {
    for (let i = 0; i < attempts; i++) {
        let result;
        if (obj.c_id && obj.o_id) {
            result = grpc_client.verifyHash(obj.c_id, obj.o_id, obj.payload_hash);
        } else if (obj.s3_bucket && obj.s3_key) {
            result = s3_client.verifyHash(obj.s3_bucket, obj.s3_key, obj.payload_hash);
        } else {
            console.log(`Object id=${obj.id} cannot be verified with supported protocols`);
            return "skipped";
        }

        if (result.success) {
            return "verified";
        } else if (result.error == "hash mismatch") {
            return "invalid";
        }

        // Unless we explicitly saw that there was a hash mismatch, then we will retry after a delay
        console.log(`Verify error on ${obj.id}: ${result.error}. Object will be re-tried`);
        sleep(__ENV.SLEEP);
    }

    return "invalid";
}
