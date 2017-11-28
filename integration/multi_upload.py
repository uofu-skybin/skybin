
# Upload multiple files and folders

from constants import *
from helpers import *
from pprint import pprint
import json
import os
import random
import requests
import string
import sys

MIN_SIZE = 1024 * 1024
MAX_SIZE = 10 * MIN_SIZE

def main():
    print(sys.argv[0])

    print('reserving space')
    reserve_space(1 << 30)

    files = [
        'a.txt',
        'b.txt',
        'c.txt'
    ]

    print('creating files')
    for name in files:
        with open('files/' + name, 'w+') as f:
            size = random.randint(MIN_SIZE, MAX_SIZE)
            for i in range(size//1024):
                s = ''.join([string.ascii_uppercase[j % len(string.ascii_uppercase)]
                             for j in range(1024)])
                f.write(s)

    print('uploading files to root folder')
    for name in files:
        upload_file(os.path.abspath('files/' + name), name)

    folders = [
        'school',
        'work',
        'pics'
    ]

    print('creating folders')
    for folder in folders:
        upload_file(None, folder)

    print('uploading files to folders')
    for folder in folders:
        for filename in files:
            upload_file(os.path.abspath('files/' + filename),
                         '{}/{}'.format(folder, filename))

    print('listing files')
    resp = requests.get(RENTER_ADDR + '/files')
    assert(resp.status_code == 200)
    resp_obj = json.loads(resp.content)
    assert('files' in resp_obj)
    names = set([f['name'] for f in resp_obj['files']])
    for filename in files:
        assert(filename in names)
    for folder in folders:
        for filename in files:
            assert('{}/{}'.format(folder, filename) in names)

    print('PASS')

if __name__ == "__main__":
    main()
