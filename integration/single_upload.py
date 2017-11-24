
# Sanity-check for basic upload/list/download functionality

from constants import *
from helpers import *
from pprint import pprint
import filecmp
import json
import os
import requests
import sys

FILE_NAME = 'files/samplefile.txt'
DOWNLOAD_NAME = 'files/samplefile_download.txt'

def main():
    print(sys.argv[0])

    print('reserving space')
    reserve_space(1 << 25)

    print('uploading file')
    with open(FILE_NAME, 'w+') as f:
        for _ in range(50):
            f.write('hello world\n')
    resp = requests.post(RENTER_ADDR + '/files', json={
        'sourcePath': os.path.abspath(FILE_NAME),
        'destPath': 'samplefile.txt'
    })
    if resp.status_code != 201:
        print('post /files. bad response {}'.format(resp))
        sys.exit(1)
    file_info = json.loads(resp.content)['file']
    print('file info:')
    pprint(file_info)
    file_id = file_info['id']

    print('listing files')
    resp = requests.get(RENTER_ADDR + '/files')
    if resp.status_code != 200:
        print('get /files. bad response: {}'.format(resp))
        sys.exit(1)
    content = json.loads(resp.content)
    print('response:')
    pprint(content)

    print('downloading file')
    url = '{}/files/{}/download'.format(RENTER_ADDR, file_id)
    destination = os.path.abspath(DOWNLOAD_NAME)
    resp = requests.post(url, json={'destination': destination})
    if resp.status_code != 201:
        print('post /files/downloads. bad response: {}'.format(resp))
    assert(os.path.isfile(DOWNLOAD_NAME))
    assert(filecmp.cmp(FILE_NAME, DOWNLOAD_NAME))
    print('downloaded file to {}'.format(destination))

if __name__ == "__main__":
    main()
