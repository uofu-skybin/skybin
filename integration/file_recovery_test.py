
"""
Test that a file can be downloaded even when
data blocks are unaccessible.
"""

import argparse
import collections
import os
import random
import filecmp
import time
from test_framework import setup_test

def test_all_blocks_inaccessible(ctxt, renter):
    renter.reserve_space(10*1024*1024*1024)
    input_file = ctxt.get_test_file()
    f = renter.upload_file(input_file, ctxt.relpath(input_file))
    for provider in ctxt.providers:
        provider.disconnect()
    try:
        renter.download_file(f['id'], ctxt.create_output_path())
        ctxt.fail('shouldnt be able to retrieve file while providers are offline')
    except:
        pass
    for provider in ctxt.providers:
        provider.restart()
    time.sleep(0.5)

def fuzz_inaccessible_blocks(ctxt, renter):
    renter.reserve_space(10*1024*1024*1024)
    input_path = ctxt.get_test_file()
    f = renter.upload_file(input_path, ctxt.relpath(input_path))
    version = f['versions'][0]
    blocks_per_addr = collections.Counter()
    for blk in version['blocks']:
        blocks_per_addr[blk['location']['address']] += 1
    poss_addrs = [k for k, v in blocks_per_addr.items() if v > 1 and v < 4]
    if len(poss_addrs) == 0:
        ctxt.log('fuzz_inaccessible_blocks: skipping due to poor block distribution')
        return
    for _ in range(10):
        addr = random.choice(poss_addrs)
        providers = [p for p in ctxt.providers if p.address == addr]
        for provider in providers:
                provider.disconnect()
        renter.download_file(f['id'], ctxt.create_output_path())
        for provider in providers:
            provider.restart()
        time.sleep(0.5)

def fuzz_lost_blocks(ctxt, renter):
    renter.reserve_space(10*1024*1024*1024)
    for _ in range(5):
        input_path = ctxt.create_test_file(size=random.randint(1, 50*1024*1024))
        file_info = renter.upload_file(input_path, ctxt.relpath(input_path))

        file_version = file_info['versions'][-1]
        data_blocks = file_version['blocks'][:file_version['numDataBlocks']]
        parity_blocks = file_version['blocks'][file_version['numDataBlocks']:]
        num_blocks_to_remove = random.randint(1, len(parity_blocks))
        num_parity_to_remove = random.randint(0, len(parity_blocks) - num_blocks_to_remove)
        data_blocks_to_remove = random.sample(data_blocks, num_blocks_to_remove)
        parity_blocks_to_remove = random.sample(parity_blocks, num_parity_to_remove)
        blocks_to_remove = data_blocks_to_remove + parity_blocks_to_remove

        # Remove the blocks. This is fragile.
        # I make an assumption about where the blocks are located
        # and manually remove them from the filesystem.
        renter_id = renter.get_info()['id']
        for block in blocks_to_remove:
            ctxt.log('removing block {}'.format(block['id']))
            location = block['location']
            addr = location['address']
            pvdr = next(p for p in ctxt.providers if p.address  == addr)
            block_location = os.path.join(pvdr.homedir, 'blocks', renter_id, block['id'])
            os.remove(block_location)

        output_path = ctxt.create_output_path()
        renter.download_file(file_info['id'], output_path)
        is_match = filecmp.cmp(input_path, output_path)
        ctxt.assert_true(is_match, 'download does not match upload after removing several blocks')

def fuzz_corrupted_blocks(ctxt, renter):
    renter.reserve_space(10*1024*1024*1024)
    for _ in range(5):
        input_path = ctxt.create_test_file(size=random.randint(1, 25*1024*1024))
        file_info = renter.upload_file(input_path, ctxt.relpath(input_path))

        file_version = file_info['versions'][-1]
        data_blocks = file_version['blocks'][:file_version['numDataBlocks']]
        parity_blocks = file_version['blocks'][file_version['numDataBlocks']:]
        num_blocks_to_corrupt = random.randint(1, len(parity_blocks))
        num_parity_to_corrupt = random.randint(0, len(parity_blocks) - num_blocks_to_corrupt)
        data_blocks_to_corrupt = random.sample(data_blocks, num_blocks_to_corrupt)
        parity_blocks_to_corrupt = random.sample(parity_blocks, num_parity_to_corrupt)
        blocks_to_corrupt = data_blocks_to_corrupt + parity_blocks_to_corrupt

        renter_id = renter.get_info()['id']
        for block in blocks_to_corrupt:
            ctxt.log('corrupting block {}'.format(block['id']))
            location = block['location']
            addr = location['address']
            pvdr = next(p for p in ctxt.providers if p.address  == addr)
            block_location = os.path.join(pvdr.homedir, 'blocks', renter_id, block['id'])
            with open(block_location, 'r+') as f:
                f.seek(random.randint(0, block['size']))
                amt = random.randint(1, 128)
                f.write(str(os.urandom(amt)))

        output_path = ctxt.create_output_path()
        renter.download_file(file_info['id'], output_path)
        is_match = filecmp.cmp(input_path, output_path)
        ctxt.assert_true(is_match, 'download does not match upload after corrupting several blocks')

def file_recovery_test(ctxt, args):
    assert len(ctxt.additional_renters) >= 4
    test_all_blocks_inaccessible(ctxt, ctxt.additional_renters[0])
    fuzz_inaccessible_blocks(ctxt, ctxt.additional_renters[1])
    fuzz_lost_blocks(ctxt, ctxt.additional_renters[2])
    fuzz_corrupted_blocks(ctxt, ctxt.additional_renters[3])

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--num_providers', type=int, default=5,
                        help='number of providers to run')
    args = parser.parse_args()
    ctxt = setup_test(
        num_providers=args.num_providers,
        num_additional_renters=4,
    )
    try:
        ctxt.log('file recovery test')
        file_recovery_test(ctxt, args)
        ctxt.log('ok')
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
