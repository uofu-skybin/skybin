
"""
Test for downloading a folder tree.
"""

import argparse
import os.path
from test_framework import setup_test

DEFAULT_FILE_SIZE = 1024 * 1024

def folder_download_test(ctxt):
    input_path = ctxt.create_test_file(size=DEFAULT_FILE_SIZE)

    # Reserve a gigabyte of space.
    ctxt.renter.reserve_space(int(1e9))

    # Create folder hierarchy
    root_folder = ctxt.renter.create_folder('folder')
    ctxt.renter.create_folder('folder/folder1')
    ctxt.renter.create_folder('folder/folder1/folder2')
    ctxt.renter.upload_file(input_path, 'folder/file1.txt')
    ctxt.renter.upload_file(input_path, 'folder/folder1/file1.txt')
    ctxt.renter.upload_file(input_path, 'folder/folder1/folder2/file1.txt')

    # Create additional files which should not be downloaded
    ctxt.renter.create_folder('other_folder')
    ctxt.renter.upload_file(input_path, 'other_file.txt')

    # Download the folder hierarchy
    output_path = ctxt.create_output_path()
    ctxt.renter.download_file(root_folder['id'], output_path)

    # Check that the proper file tree was created
    file_names = [
        output_path,
        os.path.join(output_path, 'folder1'),
        os.path.join(output_path, 'folder1', 'folder2'),
        os.path.join(output_path, 'file1.txt'),
        os.path.join(output_path, 'folder1', 'file1.txt'),
        os.path.join(output_path, 'folder1', 'folder2', 'file1.txt'),
    ]
    for name in file_names:
        ctxt.assert_true(os.path.exists(name), 'failed to download {}'.format(name))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('folder download test')
        folder_download_test(ctxt)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
