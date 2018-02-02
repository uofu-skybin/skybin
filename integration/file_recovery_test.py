
"""
Test that a file can be downloaded even when 
data blocks are unaccessible.
"""

import os
import random
import filecmp
from test_framework import setup_test

MIN_FILE_SIZE = 1
MAX_FILE_SIZE = 10*1024*1024

def file_recovery_test(ctxt):
    ctxt.log('loss test')

    ctxt.log('creating input file')
    file_size = random.randint(MIN_FILE_SIZE, MAX_FILE_SIZE)
    input_path = ctxt.create_test_file(file_size)

    ctxt.log('reserving space')
    ctxt.renter.reserve_space(2*MAX_FILE_SIZE)

    ctxt.log('uploading file')
    file_info = ctxt.renter.upload_file(input_path, input_path)

    # Select random data blocks and parity blocks to remove
    ctxt.log('selecting blocks to remove')
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
    renter_id = ctxt.renter.get_info()['id']
    for block in blocks_to_remove:
        ctxt.log('removing block {}'.format(block['id']))
        location = block['locations'][0]
        addr = location['address']
        pvdr = next(p for p in ctxt.providers if p.address  == addr)
        block_location = os.path.join(pvdr.homedir, 'provider', 'blocks', renter_id, block['id'])
        os.remove(block_location)

    # Attempt to download the file
    ctxt.log('retrieving file')
    output_path = ctxt.create_output_path()
    ctxt.renter.download_file(file_info['id'], output_path)
    is_match = filecmp.cmp(input_path, output_path)
    ctxt.assert_true(is_match, 'download does not match upload after removing several blocks')

    ctxt.log('ok')

def main():
    ctxt = setup_test(
        num_providers=3
    )
    try:
        file_recovery_test(ctxt)
    finally:
        ctxt.teardown()

if __name__ == "__main__":
    main()
