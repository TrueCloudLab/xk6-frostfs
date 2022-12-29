import {uuidv4} from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';
import {fail} from 'k6'
import s3 from 'k6/x/frostfs/s3';

const payload = open('../go.sum', 'b');
const bucket = "cats"
const s3_cli = s3.connect("https://s3.frostfs.devenv:8080", {'no_verify_ssl': 'true'})

export const options = {
    stages: [
        {duration: '30s', target: 10},
    ],
};

export function setup() {
    const params = {
        acl: 'private',
        lock_enabled: 'true',
        location_constraint: 'ru'
    }

    const res = s3_cli.createBucket(bucket, params)
    if (!res.success) {
        fail(res.error)
    }
}

export default function () {
    const key = uuidv4();
    if (s3_cli.put(bucket, key, payload).success) {
        s3_cli.get(bucket, key)
    }
}
