import re

from helpers.cmd import execute_cmd


def create_container(endpoint, policy, wallet_file, wallet_config):
    cmd_line = f"neofs-cli --rpc-endpoint {endpoint} container create --wallet {wallet_file} --config {wallet_config} " \
               f" --policy '{policy}' --basic-acl public-read-write --await"

    output, success = execute_cmd(cmd_line)

    if not success:
        print(f" > Container has not been created:\n{output}")
        return False
    else:
        try:
            fst_str = output.split('\n')[0]
        except Exception:
            print(f"Got empty output: {output}")
            return False
        splitted = fst_str.split(": ")
        if len(splitted) != 2:
            raise ValueError(f"no CID was parsed from command output: \t{fst_str}")

        print(f"Created container: {splitted[1]}")

        return splitted[1]


def upload_object(container, payload_filepath, endpoint, wallet_file, wallet_config):
    object_name = ""
    cmd_line = f"neofs-cli --rpc-endpoint {endpoint} object put --file {payload_filepath} --wallet {wallet_file} --config {wallet_config} " \
               f"--cid {container} --no-progress"
    output, success = execute_cmd(cmd_line)

    if not success:
        print(f" > Object {object_name} has not been uploaded:\n{output}")
        return False
    else:
        try:
            # taking second string from command output
            snd_str = output.split('\n')[1]
        except Exception:
            print(f"Got empty input: {output}")
            return False
        splitted = snd_str.split(": ")
        if len(splitted) != 2:
            raise Exception(f"no OID was parsed from command output: \t{snd_str}")
        return splitted[1]


def get_object(cid, oid, endpoint, out_filepath, wallet_file, wallet_config):
    cmd_line = f"neofs-cli object get -r {endpoint} --cid {cid} --oid {oid} --wallet {wallet_file} --config {wallet_config} " \
               f"--file {out_filepath}"

    output, success = execute_cmd(cmd_line)

    if not success:
        print(f" > Failed to get object {output} from container {cid} \r\n"
              f" > Error: {output}")
        return False

    return True


def search_object_by_id(cid, oid, endpoint, wallet_file, wallet_config, ttl=2):
    cmd_line = f"neofs-cli object search --ttl {ttl} -r {endpoint} --cid {cid} --oid {oid} --wallet {wallet_file} --config {wallet_config} "

    output, success = execute_cmd(cmd_line)

    if not success:
        print(f" > Failed to search object {oid} for container {cid} \r\n"
              f" > Error: {output}")
        return False

    re_rst = re.search(r'Found (\d+) objects', output)

    if not re_rst:
        raise Exception("Failed to parce search results")

    return re_rst.group(1)
