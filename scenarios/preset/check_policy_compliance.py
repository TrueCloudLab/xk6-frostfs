#!/usr/bin/python3

import argparse
import json

from argparse import Namespace
from collections import Counter
from concurrent.futures import ProcessPoolExecutor

from helpers.cmd import ProgressBar
from helpers.frostfs_cli import search_object_by_id

parser = argparse.ArgumentParser()
parser.add_argument('--endpoints', help='Node address')
parser.add_argument('--expected_copies', help="Expected amount of object copies")
parser.add_argument('--preset_file', help='JSON file path with preset')
parser.add_argument('--max_workers', help='Max workers in parallel', default=50)
parser.add_argument('--print_failed', help='Print failed objects', default=False)
parser.add_argument('--wallet', help='Wallet file path')
parser.add_argument('--config', help='Wallet config file path')


args: Namespace = parser.parse_args()
print(args)


def main():
    success_objs = 0
    failed_objs = 0

    with open(args.preset_file) as f:
        preset_text = f.read()

    preset_json = json.loads(preset_text)

    objs = preset_json.get('objects')
    objs_len = len(objs)

    endpoints = args.endpoints.split(',')
    wallet = args.wallet
    wallet_config = args.config

    final_discrubution = Counter(dict.fromkeys(endpoints, 0))

    with ProcessPoolExecutor(max_workers=50) as executor:
        search_runs = {executor.submit(check_object_amounts, obj.get('container'), obj.get('object'), endpoints,
                                       int(args.expected_copies), wallet, wallet_config): obj for obj in objs}

        ProgressBar.start()

        for run in search_runs:
            result, distribution = run.result()

            if result:
                success_objs += 1
            else:
                failed_objs += 1

            final_discrubution += distribution

            ProgressBar.print(success_objs + failed_objs, objs_len)

        ProgressBar.end()

    print(f'Success objects: {success_objs}')
    print(f'Failed objects: {failed_objs}')
    for endpoint in endpoints:
        print(f'{endpoint}: {final_discrubution[endpoint]}')


def check_object_amounts(cid, oid, endpoints, expected_copies, wallet, wallet_config):
    distribution = Counter(dict.fromkeys(endpoints, 0))

    copies_in_cluster = 0

    for endpoint in endpoints:
        copy_on_endpoint = search_object_by_id(cid, oid, endpoint, wallet, wallet_config, ttl=1)

        copies_in_cluster += int(copy_on_endpoint)

        distribution[endpoint] += int(copy_on_endpoint)

    if copies_in_cluster != expected_copies and args.print_failed:
        print(f' > Wrong copies for object {oid} in container {cid}. Copies: {copies_in_cluster}')

    return copies_in_cluster == expected_copies, distribution


if __name__ == "__main__":
    main()
