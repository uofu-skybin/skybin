
"""
Tests for upload behavior when some or all providers are offline.
"""

import argparse
import filecmp
import time
import random
from test_framework import setup_test

DEFAULT_PROVIDERS = 3
DEFAULT_RESERVATION_SIZE = 20 * 1024 * 1024 * 1024

def test_all_offline_file(ctxt):
    starting_storage = ctxt.renter.get_info()['freeStorage']
    input_path = ctxt.get_test_file()
    for provider in ctxt.providers:
        provider.disconnect()
    try:
        ctxt.renter.upload_file(input_path, ctxt.relpath(input_path))
        ctxt.fail('successfully uploaded file with no running providers')
    except:
        pass
    storage = ctxt.renter.get_info()['freeStorage']
    ctxt.assert_true(storage == starting_storage, 'failed to return storage after failed upload')
    for provider in ctxt.providers:
        provider.restart()

    # Wait for services to restart
    time.sleep(0.5)
    f = ctxt.renter.upload_file(input_path, ctxt.relpath(input_path))
    output_path = ctxt.create_output_path()
    ctxt.renter.download_file(f['id'], output_path)
    is_match = filecmp.cmp(input_path, output_path)
    ctxt.assert_true(is_match)
    ctxt.renter.remove_file(f['id'])

def test_all_offline_folder(ctxt):
    starting_storage = ctxt.renter.get_info()['freeStorage']
    starting_file_ids = [f['id'] for f in ctxt.renter.list_files()]
    root_folder = ctxt.create_test_folder()
    for _ in range(random.randint(5, 12)):
        ctxt.create_test_file(parent_folder=root_folder)
    for _ in range(random.randint(1, 3)):
        ctxt.create_test_folder(parent_folder=root_folder)
    for provider in ctxt.providers:
        provider.disconnect()
    try:
        ctxt.renter.upload_file(root_folder, ctxt.relpath(root_folder))
        ctxt.fail('successfully uploaded folder with no running provicers')
    except:
        pass
    storage = ctxt.renter.get_info()['freeStorage']
    ctxt.assert_true(storage == starting_storage, 'failed to return storage after failed upload')
    ending_file_ids = [f['id'] for f in ctxt.renter.list_files()]
    ctxt.assert_true(set(ending_file_ids) == set(starting_file_ids),
                     'files dont match the originals after a failed upload')

    # Check that we can upload the folder after restarting providers
    for provider in providers:
        provider.restart()
    time.sleep(0.5)
    f = ctxt.renter.upload_file(root_folder, ctxt.relpath(root_folder))
    output_path = ctxt.create_folder_output_path()
    ctxt.renter.download_file(f['id'], output_path)
    ctxt.renter.remove_file(f['id'], recursive=True)

# Test that folders can be uploaded even when some providers
# the renter has contracts with are offline. Note that this
# may fail if the renter has not reserved contracts with all
# providers.
def test_some_offline_some_online(ctxt):
    root_folder = ctxt.create_test_folder()
    for _ in range(random.randint(1, 15)):
        size = random.randint(4096, 8 * 1024 * 1024)
        ctxt.create_test_file(size, parent_folder=root_folder)
    for _ in range(2):
        ctxt.create_test_folder(parent_folder=root_folder)

    starting_storage = ctxt.renter.get_info()['freeStorage']

    for _ in range(10):
        num_offline = random.randint(1, len(ctxt.providers) - 1)
        offline_providers = random.sample(ctxt.providers, num_offline)
        for provider in offline_providers:
            provider.disconnect()

        ctxt.log('attempting upload with', num_offline, '/', len(ctxt.providers), 'providers offline')

        # Should succeed, even with some providers offline
        f = ctxt.renter.upload_file(root_folder, ctxt.relpath(root_folder))
        ctxt.renter.remove_file(f['id'], recursive=True)

        for provider in offline_providers:
            provider.restart()
        time.sleep(0.5)

    # Ensure storage didn't change over the course of the uploads
    ending_storage = ctxt.renter.get_info()['freeStorage']
    ctxt.assert_true(ending_storage == starting_storage)

def adverse_upload_test(ctxt, args):
    ctxt.renter.reserve_space(args.reservation_size)
    test_all_offline_file(ctxt)
    test_all_offline_folder(ctxt)
    test_some_offline_some_online(ctxt)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=DEFAULT_PROVIDERS,
                        help='number of providers to run')
    parser.add_argument('--reservation_size', type=int, default=DEFAULT_RESERVATION_SIZE,
                        help='amount of storage to reserve for the upload')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
    )
    try:
        ctxt.log('adverse upload test')
        adverse_upload_test(ctxt, args)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
