
"""
Test for upload of multiple files.
"""

import argparse
import random
import filecmp
from test_framework import setup_test

DEFAULT_NUM_FILES = 10
DEFAULT_MIN_SIZE = 1024
DEFAULT_MAX_SIZE = 10 * 1024 * 1024

def multi_upload_test(ctxt, num_files=DEFAULT_NUM_FILES,
                      min_size=DEFAULT_MIN_SIZE,
                      max_size=DEFAULT_MAX_SIZE):
    ctxt.log('multi upload test')

    ctxt.log('creating input files')
    input_files = []
    total_size = 0
    for _ in range(num_files):
        size = random.randint(min_size, max_size)
        input_files.append(ctxt.create_test_file(size))
        total_size += size

    ctxt.log('reserving space')
    # Reserve space in fragments
    frag_size = 5000000
    bytes_to_reserve = total_size * 4
    for _ in range(bytes_to_reserve//frag_size):
        ctxt.renter.reserve_space(frag_size)

    ctxt.log('uploading files')
    file_infos = []
    for name in input_files:
        file_infos.append(ctxt.renter.upload_file(name, name))

    ctxt.log('downloading files')
    for info, input_path in zip(file_infos, input_files):
        output_path = ctxt.create_output_path()
        ctxt.renter.download_file(info['id'], output_path)
        is_match = filecmp.cmp(input_path, output_path)
        ctxt.assert_true(is_match, 'download does not match upload')

    ctxt.log('ok')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    parser.add_argument('--num_files', type=int, default=DEFAULT_NUM_FILES,
                        help='number of files to upload')
    parser.add_argument('--min_size', type=int, default=DEFAULT_MIN_SIZE,
                        help='min file size')
    parser.add_argument('--max_size', type=int, default=DEFAULT_MAX_SIZE,
                        help='max file size')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
        alias='multiFileUploadTest',
    )
    try:
        multi_upload_test(
            ctxt,
            num_files=args.num_files,
            min_size=args.min_size,
            max_size=args.max_size,
        )
    finally:
        ctxt.teardown()        

if __name__ == "__main__":
    main()
