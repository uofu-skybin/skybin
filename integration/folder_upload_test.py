
"""
Test for uploading a folder hierarchy.
"""

import argparse
import os
import random
from test_framework import setup_test, create_file_name, create_test_file

def folder_upload_test(ctxt):

    # Create folder hierarchy
    base_folder = ctxt.test_file_dir + '/folder' + str(random.randint(1, 64))
    folders = [
        '/folder2',
        '/folder2/folder3',
    ]
    files = [
        '/file1.txt',
        '/file2.txt',
        '/folder2/file1.txt',
        '/folder2/folder3/file1.txt',
    ]
    os.makedirs(base_folder)
    for folder_name in folders:
        os.makedirs(base_folder + folder_name)
    for file_name in files:
        create_test_file(base_folder + file_name, 1024 * 1024)

    # Upload
    ctxt.renter.reserve_space(int(1e9))
    dest_folder = 'folder'
    root_folder = ctxt.renter.upload_file(base_folder, dest_folder)

    # Check file listing
    all_paths = folders + files
    all_paths = [dest_folder + path for path in all_paths]
    all_paths.append(dest_folder)
    all_files = ctxt.renter.list_files()['files']
    all_file_names = [f['name'] for f in all_files]
    for path in all_paths:
        ctxt.assert_true(set(all_file_names) == set(all_paths), 'folder hierarchies do not match')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
        remove_test_files=False,
    )
    try:
        ctxt.log('folder upload test')
        folder_upload_test(ctxt)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
