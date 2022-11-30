<p align="center">
<img src="./.github/logo.svg" width="500px" alt="NeoFS">
</p>
<p align="center">
  <a href="https://go.k6.io/k6">k6</a> extension to test and benchmark <a href="https://fs.neo.org">NeoFS</a> related protocols.
</p>

---
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

# xk6-neofs

# Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- Go
- Git

1. Install `xk6` framework for extending `k6`:
```shell
go install go.k6.io/xk6/cmd/xk6@latest
```

2. Clone this repository
```shell
git clone github.com/nspcc-dev/xk6-neofs
cd xk6-neofs
```

3. Build the binary:
```shell
xk6 build --with github.com/nspcc-dev/xk6-neofs=.
```

4. Run k6:
```shell
./k6 run test-script.js
```

# API

## Native

Create native client with `connect` method. Arguments:
- neofs storage node endpoint
- hex encoded private key (empty value produces random key)

```js
import native from 'k6/x/neofs/native';
const neofs_cli = native.connect("s01.neofs.devenv:8080", "")
```

### Methods
- `putContainer(params)`. The `params` is a dictionary (e.g. 
  `{acl:'public-read-write',placement_policy:'REP 3',name:'container-name',name_global_scope:'false'}`). 
  Returns dictionary with `success`
  boolean flag, `container_id` string, and `error` string.
- `setBufferSize(size)`. Sets internal buffer size for data upload and 
  download. Default is 64 KiB.
- `put(container_id, headers, payload)`. Returns dictionary with `success` 
  boolean flag, `object_id` string, and `error` string.
- `get(container_id, object_id)`. Returns dictionary with `success` boolean
  flag, and `error` string.
- `onsite(container_id, payload)`. Returns NeoFS object instance with prepared
  headers. Invoke `put(headers)` method on this object to upload it into NeoFS.
  It returns dictionary with `success` boolean flag, `object_id` string and
  `error` string.

## S3

Create s3 client with `connect` method. Arguments:
- s3 gateway endpoint

Credentials are taken from default AWS configuration files and ENVs.

```js
import s3 from 'k6/x/neofs/s3';
const s3_cli = s3.connect("http://s3.neofs.devenv:8080")
```

You can also provide additional options:
```js
import s3 from 'k6/x/neofs/s3';
const s3_cli = s3.connect("http://s3.neofs.devenv:8080", {'no_verify_ssl': 'true', 'timeout': '60s'})
```

* `no_verify_ss` - Bool. If `true` - skip verifying the s3 certificate chain and host name (useful if s3 uses self-signed certificates)
* `timeout` - Duration. Set timeout for requests (in http client). If omitted or zero - timeout is infinite.

### Methods
- `createBucket(bucket, params)`. Returns dictionary with `success` boolean flag
  and `error` string. The `params` is a dictionary (e.g. `{acl:'private',lock_enabled:'true',location_constraint:'ru'}`)
- `put(bucket, key, payload)`. Returns dictionary with `success` boolean flag 
  and `error` string.
- `get(bucket, key)`. Returns dictionary with `success` boolean flag and `error`
  string.

# Examples

See native protocol and s3 test suit examples in [examples](./examples) dir.

# License

- [GNU General Public License v3.0](LICENSE)
