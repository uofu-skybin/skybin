
# Sends a continous, randomized stream requests to a provider
# node at a modest rate (e.g. 1 request/5 seconds).

import sys
import base64
import requests
import json
import random
import string
import time
from pprint import pprint

NUM_RENTERS = 5
MAX_STORAGE = 1 << 24 # Maximum storage used per-renter
MIN_BLOCK_SIZE = 1 << 9
MAX_BLOCK_SIZE = 1 << 10
REQ_FREQUENCY = 5 # seconds

def randstr(size):
    return ''.join(random.choice(string.ascii_uppercase) for _ in range(size))

class Provider:

    def __init__(self, addr):
        self._addr = addr

    def negotiate(self, contract):
        url = '{}/contracts'.format(self._addr)
        resp = requests.post(url, json={'contract': contract})
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))

    def put_block(self, renter_id, block_id, data):
        url = '{}/blocks/{}'.format(self._addr, block_id)

        # Go's JSON deserializer complained without base64 encoding the data.
        dstr = base64.b64encode(data.encode('utf-8')).decode('utf-8')
        resp = requests.post(url, json={'renterId': renter_id, 'data': dstr})
        if resp.status_code != 201:
            raise ValueError(resp.content.decode('utf-8'))

    def get_block(self, id):
        url = '{}/blocks/{}'.format(self._addr, id)
        resp = requests.get(url)
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))
        msg = json.loads(resp.content)
        return msg['data']

    def delete_block(self, id):
        url = '{}/blocks/{}'.format(self._addr, id)
        resp = requests.delete(url)
        if resp.status_code != 200:
            raise ValueError(resp.content.decode('utf-8'))

def gen_renter():
    return {
        'id': randstr(10),
        'blocks': set(),
        'num_contracts': 0,
        'storage_reserved': 0,
        'storage_used': 0,
    }

# A block is an (ID, size) tuple.
def id(block):
    return block[0]

def size(block):
    return block[1]

def one_round(renter, provider):

    r = random.randint(0, 100)

    # Create contract
    if r < 10 or renter['storage_reserved'] - renter['storage_used'] <= 0:
        contract = {
            'renterId': renter['id'],
            'providerId': '123456789',
            'storageSpace': random.randint(1e3, 1e9),
            'renterSignature': '123456789',
        }
        provider.negotiate(contract)
        renter['num_contracts'] += 1
        renter['storage_reserved'] += contract['storageSpace']
        return

    # Put block
    if r < 40 or len(renter['blocks']) < 5:
        block_id = randstr(10)
        max_size = min(MAX_BLOCK_SIZE, renter['storage_reserved'] - renter['storage_used'])
        block_size = random.randint(MIN_BLOCK_SIZE, max_size)
        data = randstr(block_size)
        provider.put_block(renter['id'], block_id, data)
        renter['blocks'].add((block_id, len(data)))
        renter['storage_used'] += len(data)
        return

    # Delete block
    if r < 60 or renter['storage_used'] >= MAX_STORAGE:
        block = random.choice(list(renter['blocks']))
        provider.delete_block(id(block))
        renter['blocks'].remove(block)
        renter['storage_used'] -= size(block)
        return

    # Get block
    block = random.choice(list(renter['blocks']))
    provider.get_block(id(block))

def main():

    if len(sys.argv) != 2:
        print('usage: {} <address>'.format(sys.argv[0]))
        print('example: {} http://localhost:8003'.format(sys.argv[0]))
        sys.exit(1)

    addr = sys.argv[1]
    provider = Provider(addr)
    renters = [gen_renter() for _ in range(NUM_RENTERS)]
    roundno = 0
    while True:
        renter = random.choice(renters)
        try:
            one_round(renter, provider)
        except Exception as e:
            print('Error:', e)
        time.sleep(REQ_FREQUENCY)
        roundno += 1
        if roundno % 10 == 1:
            ncontracts = 0
            nblocks = 0
            storage_reserved = 0
            storage_used = 0
            for renter in renters:
                ncontracts += renter['num_contracts']
                nblocks += len(renter['blocks'])
                storage_reserved += renter['storage_reserved']
                storage_used += renter['storage_used']
            print('ROUND {} nrenters={} ncontracts={} nblocks={} storage_reserved={} storage_used={}'.format(
                roundno, len(renters), ncontracts, nblocks, storage_reserved, storage_used))

if __name__ == "__main__":
    main()
