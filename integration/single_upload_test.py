
"""
Test for upload of single file.
"""

import argparse
import filecmp
from test_framework import setup_test

DEFAULT_FILE_SIZE = 1024 * 1024
DEFAULT_RESERVATION_SIZE = 2 * 1024 * 1024 * 1024

def single_upload_test(ctxt,
                       file_size=DEFAULT_FILE_SIZE,
                       reservation_size=DEFAULT_RESERVATION_SIZE):
    ctxt.renter.reserve_space(reservation_size)
    input_path = ctxt.create_test_file(size=file_size)
    file_info = ctxt.renter.upload_file(source=input_path, dest=input_path)
    output_path = ctxt.create_output_path()
    ctxt.renter.download_file(file_info['id'], output_path)
    is_match = filecmp.cmp(input_path, output_path)
    ctxt.assert_true(is_match, 'download does not match upload')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    parser.add_argument('--file_size', type=int, default=DEFAULT_FILE_SIZE,
                        help='file size to upload')
    parser.add_argument('--reservation_size', type=int, default=DEFAULT_RESERVATION_SIZE,
                        help='amount of storage to reserve for the upload')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('single upload test')
        single_upload_test(
            ctxt,
            file_size=args.file_size,
            reservation_size=args.reservation_size,
        )
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
