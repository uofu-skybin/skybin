
"""
Test for uploading, downloading, and removing versions of a file with the same name.
"""

import argparse
import filecmp
from test_framework import setup_test

def version_test(ctxt):
    file1 = ctxt.create_test_file(size=1000)
    file2 = ctxt.create_test_file(size=1000)

    ctxt.renter.reserve_space(2 * int(1e9))

    dest_path = 'destination.txt'
    f = ctxt.renter.upload_file(file1, dest_path)

    # Upload a copy with overwrite set
    f = ctxt.renter.upload_file(file1, dest_path, should_overwrite=True)
    ctxt.assert_true(len(f['versions']) == 1, 'failed to overwrite old version')

    # Upload the second file with the same destination and without overwrite set
    f = ctxt.renter.upload_file(file2, dest_path)
    ctxt.assert_true(len(f['versions']) == 2, 'failed to create new version')

    all_files = ctxt.renter.list_files()
    ctxt.assert_true(len(all_files) == 1, 'created new file unexpectedly')

    # Download should by default download the latest version
    output_path = ctxt.create_output_path()
    ctxt.renter.download_file(f['id'], output_path)
    is_match = filecmp.cmp(file2, output_path)
    ctxt.assert_true(is_match, 'downloaded incorrect version of file')

    # Download the first version
    first_version = f['versions'][0]['num']
    ctxt.renter.download_file(f['id'], output_path, version_num=first_version)
    is_match = filecmp.cmp(file1, output_path)
    ctxt.assert_true(is_match, 'downloaded incorrect version of file')

    # Remove the first version
    ctxt.renter.remove_file(f['id'], version_num=first_version)
    all_files = ctxt.renter.list_files()
    ctxt.assert_true(len(all_files) == 1, 'removing version removed entire file')

    # Attempting to remove the final version should fail
    f = all_files[0]
    try:
        ctxt.renter.remove_file(f['id'], version_num=f['versions'][0]['versionNum'])
        ctxt.assert_true(False, 'removing final file version should not succeed')
    except:
        pass

    # Removing the file altogether should succeed
    ctxt.renter.remove_file(f['id'])

    # Finally, uploading five new copies should create a file with five versions
    for _ in range(5):
        f = ctxt.renter.upload_file(file1, dest_path)

    ctxt.assert_true(len(f['versions']) == 5, 'file has too few versions')

    # ...And deleting the file should delete all versions
    ctxt.renter.remove_file(f['id'])
    all_files = ctxt.renter.list_files()
    ctxt.assert_true(len(all_files) == 0, 'renter should not have any files')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('version test')
        version_test(ctxt)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == '__main__':
    main()
