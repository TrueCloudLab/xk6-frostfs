#!/usr/bin/python3

import argparse
import json

from helpers.neofs_cli import get_object

parser = argparse.ArgumentParser()
parser.add_argument('--endpoint', help='Node address')
parser.add_argument('--wallet', help='Wallet file path')
parser.add_argument('--config', help='Wallet config file path')
parser.add_argument('--preset_file', help='JSON file path with preset')

args = parser.parse_args()


def main():
    with open(args.preset_file) as f:
        preset_text = f.read()

    preset = json.loads(preset_text)

    success_objs = 0
    failed_objs = 0

    wallet = args.wallet
    wallet_config = args.config

    for obj in preset.get('objects'):
        oid = obj.get('object')
        cid = obj.get('container')

        rst = get_object(cid, oid, args.endpoint, "/dev/null", wallet, wallet_config)

        if rst:
            success_objs += 1
        else:
            failed_objs += 1

    print(f'Success objects: {success_objs}')
    print(f'Failed objects: {failed_objs}')


if __name__ == "__main__":
    main()
