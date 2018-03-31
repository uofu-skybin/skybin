
"""
Tests for removing files and folders.
"""

import argparse
import random
import os
from test_framework import setup_test

def test_simple_removal(ctxt):
    starting_space = ctxt.renter.get_info()['freeStorage']
    starting_files = len(ctxt.renter.list_files())
    input_file = ctxt.get_test_file()
    f = ctxt.renter.upload_file(input_file, 'file.txt')
    ctxt.renter.remove_file(f['id'])
    ctxt.assert_true(len(ctxt.renter.list_files()) == starting_files)
    try:
        ctxt.renter.get_file(f['id'])
        ctxt.fail('shouldnt be able to retrieve removed file')
    except:
        pass
    try:
        ctxt.renter.download_file(f['id'])
        ctxt.fail('shouldnt be able to download removed file')
    except:
        pass
    ctxt.assert_true(ctxt.renter.get_info()['totalFiles'] == 0)
    ctxt.assert_true(ctxt.renter.get_info()['freeStorage'] == starting_space)

def test_multiple_removals(ctxt):
    starting_space = ctxt.renter.get_info()['freeStorage']
    starting_files = ctxt.renter.get_info()['totalFiles']
    for _ in range(3):
        input_file = ctxt.get_test_file()
        ctxt.renter.upload_file(input_file, ctxt.create_file_name())

    files = ctxt.renter.list_files()
    ctxt.assert_true(len(files) == starting_files + 3)
    ctxt.renter.remove_file(files[0]['id'])

    files = ctxt.renter.list_files()
    ctxt.assert_true(len(files) == starting_files + 2)
    ctxt.renter.remove_file(files[0]['id'])

    files = ctxt.renter.list_files()
    ctxt.assert_true(len(files) == 1)
    ctxt.renter.remove_file(files[0]['id'])

    ctxt.assert_true(len(ctxt.renter.list_files()) == 0)
    ctxt.assert_true(ctxt.renter.get_info()['freeStorage'] == starting_space)
    ctxt.assert_true(ctxt.renter.get_info()['totalFiles'] == starting_files)

def test_remove_version(ctxt):
    input_file = ctxt.get_test_file()
    dest_path = 'foo.txt'
    for _ in range(3):
        f = ctxt.renter.upload_file(input_file, dest_path, should_overwrite=False)

    ctxt.assert_true(len(f['versions']) == 3)
    vnum = f['versions'][-1]['num']
    ctxt.renter.remove_file(f['id'], version_num=vnum)

    f = ctxt.renter.get_file(f['id'])
    ctxt.assert_true(len(f['versions']) == 2)
    ctxt.assert_true(vnum not in [v['num'] for v in f['versions']])
    vnum = f['versions'][0]['num']
    ctxt.renter.remove_file(f['id'], version_num=vnum)

    f = ctxt.renter.get_file(f['id'])
    ctxt.assert_true(vnum not in [v['num'] for v in f['versions']])
    ctxt.renter.remove_file(f['id'])

def test_remove_invalid_version(ctxt):
    input_file = ctxt.create_test_file(size=1024)
    for _ in range(4):
        f = ctxt.renter.upload_file(input_file, 'file.txt')
    vnum = max([v['num'] for v in f['versions']]) + 1
    try:
        ctxt.renter.remove_file(f['id'], version_num=vnum)
        ctxt.fail('removed nonexistent version')
    except:
        pass
    vnum = min(v['num'] for v in f['versions']) - 1
    try:
        ctxt.renter.remove_file(f['id'], version_num=vnum)
        ctxt.fail('removed nonexistent version')
    except:
        pass
    ctxt.renter.remove_file(f['id'])

def test_remove_folder(ctxt):
    starting_files = len(ctxt.renter.list_files())

    # Upload a folder tree
    base_folder = ctxt.create_test_folder()
    for _ in range(3):
        ctxt.create_test_file(parent_folder=base_folder)
    nested_folder = ctxt.create_test_folder(parent_folder=base_folder)
    for _ in range(2):
        ctxt.create_test_file(parent_folder=nested_folder)
    f = ctxt.renter.upload_file(base_folder, 'folder')

    # Upload an unrelated file
    input_file = ctxt.get_test_file()
    skybin_file = ctxt.renter.upload_file(input_file, 'file.txt')

    # Remove the folder
    ctxt.renter.remove_file(f['id'], recursive=True)

    files = ctxt.renter.list_files()
    names = [f['name'] for f in files]
    ctxt.assert_true(len(files) == starting_files + 1)
    ctxt.assert_true('file.txt' in names)

    ctxt.renter.remove_file(skybin_file['id'])

def test_remove_folder_version(ctxt):
    folder = ctxt.create_test_folder()
    f = ctxt.renter.upload_file(folder, 'folder')
    try:
        ctxt.renter.remove_file(f['id'], version_num=0)
        ctxt.fail('shouldnt be able to remove folder version')
    except:
        pass
    try:
        ctxt.renter.remove_file(f['id'], version_num=1)
        ctxt.fail('shouldnt be able to remove folder version')
    except:
        pass
    ctxt.renter.remove_file(f['id'])

def test_remove_folder_with_wrong_option(ctxt):

    # Attempt removing a non-empty folder without the recursive option
    folder = ctxt.create_test_folder()
    child  = ctxt.create_test_file(parent_folder=folder)
    ctxt.renter.upload_file(folder, ctxt.relpath(folder))
    try:
        ctxt.renter.remove_file(folder)
        ctxt.fail('shouldnt be able to remove non-empty folder without recursive option')
    except:
        pass
    files = ctxt.renter.list_files()
    file_names = [f['name'] for f in files]
    ctxt.assert_true(ctxt.relpath(folder) in file_names)
    ctxt.assert_true(ctxt.relpath(child) in file_names)

def test_remove_nested_folder(ctxt):

    # Upload a folder tree
    base_folder = ctxt.create_test_folder()
    for _ in range(3):
        ctxt.create_test_file(parent_folder=base_folder)
    nested1 = ctxt.create_test_folder(parent_folder=base_folder)
    ctxt.create_test_file(parent_folder=nested1)
    f = ctxt.renter.upload_file(base_folder, ctxt.relpath(base_folder))
    base_folder_id = f['id']

    expected_ids = [f['id'] for f in ctxt.renter.list_files()]

    # Upload an additional nested folder tree
    nested2 = ctxt.create_test_folder(parent_folder=base_folder)
    ctxt.create_test_file(parent_folder=nested2)
    f = ctxt.renter.upload_file(nested2, ctxt.relpath(nested2))

    # Remove the nested folder
    ctxt.renter.remove_file(f['id'], recursive=True)

    # Ensure the set of remaining files matches the original set
    actual_ids = [f['id'] for f in ctxt.renter.list_files()]
    ctxt.assert_true(set(expected_ids) == set(actual_ids))

    ctxt.renter.remove_file(base_folder_id, recursive=True)

def rm_file_test(ctxt):
    ctxt.renter.reserve_space(1 * 1024 * 1024 * 1024)
    test_simple_removal(ctxt)
    test_multiple_removals(ctxt)
    test_remove_version(ctxt)
    test_remove_invalid_version(ctxt)
    test_remove_folder(ctxt)
    test_remove_folder_with_wrong_option(ctxt)
    test_remove_folder_version(ctxt)
    test_remove_nested_folder(ctxt)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=3,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('rm file test')
        rm_file_test(ctxt)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
