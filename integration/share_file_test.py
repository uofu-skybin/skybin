
"""
Test sharing a single file.
"""

import argparse
import filecmp
from test_framework import setup_test

DEFAULT_FILE_SIZE = 1024 * 1024

def share_test(ctxt, file_size=DEFAULT_FILE_SIZE):
    input_path = ctxt.create_test_file(size=file_size)

    # Reserve ample space.
    ctxt.renter.reserve_space(int(2*1e9))
    file_info = ctxt.renter.upload_file(source=input_path, dest=input_path)

    # Share the file with another renter
    renter_alias = ctxt.additional_renters[0].get_info()['alias']
    ctxt.renter.share_file(file_info['id'], renter_alias)

    # Check that the operation updated the file's permissions
    file_info = [f for f in ctxt.renter.list_files() if f['id'] == file_info['id']][0]
    ctxt.assert_true(len(file_info['accessList']) == 1)
    ctxt.assert_true(file_info['accessList'][0]['renterAlias'] == renter_alias)

    # Attempt to retrieve the file with the other renter
    output_path = ctxt.create_output_path()
    ctxt.additional_renters[0].download_file(file_info['id'], output_path)
    is_match = filecmp.cmp(input_path, output_path)
    ctxt.assert_true(is_match, 'download does not match upload')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    parser.add_argument('--file_size', type=int, default=DEFAULT_FILE_SIZE,
                        help='file size to upload')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
        num_additional_renters=1,
    )
    try:
        ctxt.log('share test')
        share_test(ctxt, file_size=args.file_size)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
