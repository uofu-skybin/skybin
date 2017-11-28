
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

    resp = upload_file(os.path.abspath(FILE_NAME), 'samplefile.txt')
    file_info = resp['file']
    file_id = file_info['id']

    print('downloading file')
    destination = os.path.abspath(DOWNLOAD_NAME)
    download_file(file_id, destination)
    assert(os.path.isfile(DOWNLOAD_NAME))
    assert(filecmp.cmp(FILE_NAME, DOWNLOAD_NAME))
    print('downloaded file to {}'.format(destination))
    print('PASS')

if __name__ == "__main__":
    main()
