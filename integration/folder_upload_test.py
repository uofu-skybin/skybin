
"""
Test for uploading a folder hierarchy.
"""

import argparse
import filecmp
import os
import random
from test_framework import setup_test, create_file_name, create_test_file

def create_folder_tree(ctxt, base_folder, args):
    queue = [(base_folder, 0)]
    while len(queue) > 0:
        folder, depth = queue.pop(0)
        if depth == args.folder_depth:
            continue
        for _ in range(args.files_per_folder):
            file_size = random.randint(args.min_file_size, args.max_file_size)
            _ = ctxt.create_test_file(size=file_size, parent_folder=folder)
        for _ in range(args.num_sub_folders):
            child_folder = ctxt.create_test_folder(parent_folder=folder)
            queue.append((child_folder, depth + 1))

def create_test_folders(ctxt, args):
    folders = []
    for _ in range(args.num_folders):
        base_folder = ctxt.create_test_folder()
        create_folder_tree(ctxt, base_folder, args)
        folders.append(base_folder)
    return folders

def folder_upload_test(ctxt, args):
    ctxt.renter.reserve_space(args.reservation_size)
    source_folders = create_test_folders(ctxt, args)
    skybin_folders = []
    for i, folder in enumerate(source_folders):
        f = ctxt.renter.upload_file(folder, 'folder' + str(i))
        skybin_folders.append(f)

    for source_folder, skybin_folder in zip(source_folders, skybin_folders):
        output_path = ctxt.create_folder_output_path()
        ctxt.renter.download_file(skybin_folder['id'], output_path)
        dir_diff = filecmp.dircmp(source_folder, output_path)
        if len(dir_diff.left_only) > 0 or len(dir_diff.right_only) > 0:
            ctxt.assert_true(False, 'Folder hierarchy mismatch')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    parser.add_argument('--reservation_size', type=int, default=3*1024*1024*1024,
                        help='amount of storage to reserve')    
    parser.add_argument('--num_folders', type=int, default=1,
                        help='number of top-level folders to upload')
    parser.add_argument('--num_sub_folders', type=int, default=1,
                        help='number of sub-folders per folder')
    parser.add_argument('--folder_depth', type=int, default=2,
                        help='depth of folder hierarchy')
    parser.add_argument('--files_per_folder', type=int, default=3,
                        help='files per folder')
    parser.add_argument('--min_file_size', type=int, default=1024*1024,
                        help='min file size')
    parser.add_argument('--max_file_size', type=int, default=5*1024*1024,
                        help='max file size')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('folder upload test')
        folder_upload_test(ctxt, args)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
