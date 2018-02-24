
"""
Test for concurrent uploads by a single renter
"""

import multiprocessing as mp
import argparse
import filecmp
from test_framework import setup_test

DEFAULT_FILE_SIZE = 1024 * 1024

def concurrent_upload_test(ctxt, file_size=DEFAULT_FILE_SIZE):
    ctxt.log('concurrent upload test')

    processes = [mp.Process(target=upload_file, args=(ctxt, x, file_size)) for x in range(10)]
    for p in processes:
        p.start()
    for p in processes:
        p.join()

    ctxt.log('ok')

def upload_file(ctxt, proc_num, file_size):
    input_path = ctxt.create_test_file(size=file_size)

    # Reserve ample space.
    space = file_size * 3
    ctxt.renter.reserve_space(space)
    file_info = ctxt.renter.upload_file(source=input_path, dest=input_path)
    output_path = ctxt.create_output_path()
    ctxt.renter.download_file(file_info['id'], output_path)
    is_match = filecmp.cmp(input_path, output_path)
    ctxt.assert_true(is_match, 'download does not match upload for process ' + str(proc_num))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    parser.add_argument('--file_size', type=int, default=DEFAULT_FILE_SIZE,
                        help='file size to upload')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        concurrent_upload_test(ctxt, file_size=args.file_size)
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
