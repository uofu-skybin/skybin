
"""
Tests that a file can be removed, and that storage
used by the file is reclaimed.
"""

import argparse
import random
from test_framework import setup_test

def rm_file_test(ctxt):
    ctxt.log('rm file test')

    ctxt.log('reserving space')
    file_size = random.randint(1, 10*1024*1024)
    input_file = ctxt.create_test_file(file_size)
    ctxt.renter.reserve_space(file_size*3)

    # Upload until we run out of storage
    ctxt.log('uploading files')
    file_infos = []
    for i in range(10):
        try:
            upload_name = '{}-{}'.format(input_file, i)
            file_info = ctxt.renter.upload_file(input_file, upload_name)
            file_infos.append(file_info)
        except Exception as err:
            break

    # Remove file
    ctxt.log('removing file')
    ctxt.assert_true(len(file_infos) > 0, 'ran out of storage too early')
    file_to_remove = file_infos[0]
    ctxt.renter.remove_file(file_to_remove['id'])

    # Check it doesn't appear in file list
    file_list = ctxt.renter.list_files()['files']
    print(file_list)
    exists = any([f for f in file_list if f['id'] == file_to_remove['id']])
    ctxt.assert_true(not exists, 'file appeared in listed files after removal')

    # Check that it can't be downloaded
    try:
        ctxt.renter.download_file(file_to_remove['id'], ctxt.create_output_path())
        ctxt.assert_true(False, 'downloaded file after removing it')
    except Exception:
        pass

    # Ensure space can be reused
    upload_name = '{}-{}'.format(input_file, 11)
    ctxt.renter.upload_file(input_file, upload_name)

    ctxt.log('ok')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=3,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        rm_file_test(ctxt)
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
