
"""
Test to check that download info returned from the renter service's
 /files/download endpoint includes the proper fields.
"""

import argparse
from test_framework import setup_test

def single_file_test(ctxt):
    input_path = ctxt.create_test_file(size=10*1024)
    ctxt.renter.reserve_space(int(2*1e9))
    upload_name = 'file1.txt'
    file_info = ctxt.renter.upload_file(input_path, upload_name)
    output_path = ctxt.create_output_path()
    download_info = ctxt.renter.download_file(file_info['id'], output_path)
    ctxt.assert_true(len(download_info['files']) == 1)
    finfo = download_info['files'][0]
    ctxt.assert_true(finfo['fileId'] == file_info['id'])
    ctxt.assert_true(finfo['destPath'] == output_path)
    ctxt.assert_true(finfo['name'] == upload_name)
    reported_block_ids = [block['blockId'] for block in finfo['blocks']]
    all_block_ids = [block['id'] for block in file_info['versions'][0]['blocks']]
    for id in reported_block_ids:
        ctxt.assert_true(id in all_block_ids)

def folder_test(ctxt):
    folder = ctxt.create_test_folder()
    file1 = ctxt.create_test_file(size=10*1024, parent_folder=folder)
    file2 = ctxt.create_test_file(size=10*1024, parent_folder=folder)
    ctxt.renter.reserve_space(int(2*1e9))
    folder_info = ctxt.renter.upload_file(folder, 'folder')
    output_path = ctxt.create_output_path()
    download_info = ctxt.renter.download_file(folder_info['id'], output_path)
    import pprint
    pprint.pprint(download_info)
    ctxt.assert_true(len(download_info['files']) == 3)
    all_files = ctxt.renter.list_files()
    all_names = set([f['name'] for f in all_files])
    all_ids = set([f['id'] for f in all_files])
    reported_names = set(f['name'] for f in download_info['files'])
    reported_ids = set(f['fileId'] for f in download_info['files'])
    ctxt.assert_true(reported_names.issubset(all_names))
    ctxt.assert_true(reported_ids.issubset(all_ids))

def download_info_test(ctxt):
    single_file_test(ctxt)
    folder_test(ctxt)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=1,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('download info test')
        download_info_test(ctxt)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
