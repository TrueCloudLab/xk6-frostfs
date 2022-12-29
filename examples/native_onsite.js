import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';
import native from 'k6/x/frostfs/native';

const payload = open('../go.sum', 'b');
const container = "AjSxSNNXbJUDPqqKYm1VbFVDGCakbpUNH8aGjPmGAH3B"
const frostfs_cli = native.connect("s01.frostfs.devenv:8080", "")
const frostfs_obj = frostfs_cli.onsite(container, payload)

export const options = {
    stages: [
        { duration: '30s', target: 10 },
    ],
};

export default function () {
    let headers = {
       'unique_header': uuidv4()
    }
    let resp = frostfs_obj.put(headers)
    if (resp.success) {
       frostfs_cli.get(container, resp.object_id)
    } else {
        console.log(resp.error)
    }
}
