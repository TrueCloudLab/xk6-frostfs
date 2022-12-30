#!/usr/bin/python3

import argparse
import json
import sys
from concurrent.futures import ProcessPoolExecutor

from helpers.cmd import random_payload
from helpers.aws_cli import create_bucket, upload_object

parser = argparse.ArgumentParser()

parser.add_argument('--size', help='Upload objects size in kb.')
parser.add_argument('--buckets', help='Number of buckets to create.')
parser.add_argument('--out', help='JSON file with output.')
parser.add_argument('--preload_obj', help='Number of pre-loaded objects.')
parser.add_argument('--endpoint', help='S3 Gateway address.')
parser.add_argument('--update', help='True/False, False by default. Save existed buckets from target file (--out). '
                                     'New buckets will not be created.')
parser.add_argument('--location', help='AWS location. Will be empty, if has not be declared.', default="")
parser.add_argument('--versioning', help='True/False, False by default.')
parser.add_argument('--ignore-errors', help='Ignore preset errors')
parser.add_argument('--workers', help='Count of workers in preset. Max = 50, Default = 50', default=50)

args = parser.parse_args()
print(args)

ERROR_NO_BUCKETS = 1
ERROR_NO_OBJECTS = 2
MAX_WORKERS = 50

def main():
    bucket_list = []
    objects_list = []
    payload_filepath = '/tmp/data_file'
    ignore_errors = True if args.ignore_errors else False

    workers = int(args.workers)
    if args.update:
        # Open file
        with open(args.out) as f:
            data_json = json.load(f)
            bucket_list = data_json['buckets']
        # Get CID list
    else:
        print(f"Create buckets: {args.buckets}")

        with ProcessPoolExecutor(max_workers=min(MAX_WORKERS, workers)) as executor:
            buckets_runs = {executor.submit(create_bucket, args.endpoint, args.versioning,
                                            args.location): _ for _ in range(int(args.buckets))}

        for run in buckets_runs:
            if run.result() is not None:
                bucket_list.append(run.result())

        print("Create buckets: Completed")

    print(f" > Buckets: {bucket_list}")
    if not bucket_list:
        print("No buckets to work with")
        if not ignore_errors:
            sys.exit(ERROR_NO_BUCKETS)

    print(f"Upload objects to each bucket: {args.preload_obj} ")
    random_payload(payload_filepath, args.size)
    print(" > Create random payload: Completed")

    for bucket in bucket_list:
        print(f" > Upload objects for bucket {bucket}")
        with ProcessPoolExecutor(max_workers=min(MAX_WORKERS, workers)) as executor:
            objects_runs = {executor.submit(upload_object, bucket, payload_filepath,
                                            args.endpoint): _ for _ in range(int(args.preload_obj))}

        for run in objects_runs:
            if run.result() is not None:
                objects_list.append({'bucket': bucket, 'object': run.result()})
        print(f" > Upload objects for bucket {bucket}: Completed")

    print("Upload objects to each bucket: Completed")

    if int(args.preload_obj) > 0 and not objects_list:
        print("No objects were uploaded")
        if not ignore_errors:
            sys.exit(ERROR_NO_OBJECTS)

    data = {'buckets': bucket_list, 'objects': objects_list, 'obj_size': args.size + " Kb"}

    with open(args.out, 'w+') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)

    print("Result:")
    print(f" > Total Buckets has been created: {len(bucket_list)}.")
    print(f" > Total Objects has been created: {len(objects_list)}.")


if __name__ == "__main__":
    main()
