#!/usr/bin/python3

import argparse
import json
import random

from argparse import Namespace
from concurrent.futures import ProcessPoolExecutor

from helpers.cmd import random_payload
from helpers.frostfs_cli import create_container, upload_object

parser = argparse.ArgumentParser()
parser.add_argument('--size', help='Upload objects size in kb')
parser.add_argument('--containers', help='Number of containers to create')
parser.add_argument('--out', help='JSON file with output')
parser.add_argument('--preload_obj', help='Number of pre-loaded objects')
parser.add_argument('--wallet', help='Wallet file path')
parser.add_argument('--config', help='Wallet config file path')
parser.add_argument(
    "--policy",
    help="Container placement policy",
    default="REP 2 IN X CBF 2 SELECT 2 FROM * AS X"
)
parser.add_argument('--endpoint', help='Node address')
parser.add_argument('--update', help='Save existed containers')

args: Namespace = parser.parse_args()
print(args)


def main():
    container_list = []
    objects_struct = []
    payload_filepath = '/tmp/data_file'

    endpoints = args.endpoint.split(',')

    wallet = args.wallet
    wallet_config = args.config

    if args.update:
        # Open file
        with open(args.out) as f:
            data_json = json.load(f)
            container_list = data_json['containers']
    else:
        print(f"Create containers: {args.containers}")
        with ProcessPoolExecutor(max_workers=50) as executor:
            containers_runs = {executor.submit(create_container, endpoints[random.randrange(len(endpoints))],
                                               args.policy, wallet, wallet_config): _ for _ in range(int(args.containers))}

        for run in containers_runs:
            if run.result():
                container_list.append(run.result())

        print("Create containers: Completed")

    print(f" > Containers: {container_list}")
    if not container_list:
        return

    print(f"Upload objects to each container: {args.preload_obj} ")
    random_payload(payload_filepath, args.size)
    print(" > Create random payload: Completed")

    for container in container_list:
        print(f" > Upload objects for container {container}")
        with ProcessPoolExecutor(max_workers=50) as executor:
            objects_runs = {executor.submit(upload_object, container, payload_filepath,
                              endpoints[random.randrange(len(endpoints))], wallet, wallet_config): _ for _ in range(int(args.preload_obj))}

        for run in objects_runs:
            if run.result():
                objects_struct.append({'container': container, 'object': run.result()})
        print(f" > Upload objects for container {container}: Completed")

    print("Upload objects to each container: Completed")

    data = {'containers': container_list, 'objects': objects_struct, 'obj_size': args.size + " Kb"}

    with open(args.out, 'w+') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)

    print(f"Result:")
    print(f" > Total Containers has been created: {len(container_list)}.")
    print(f" > Total Objects has been created: {len(objects_struct)}.")


if __name__ == "__main__":
    main()
