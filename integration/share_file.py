
import os
import requests
import sys
import json
from helpers import *
from constants import *

def main():
    print(sys.argv[0])

    print('reserving space')
    reserve_space(1 << 30)

    print('uploading file')
    with open('files/share.txt', 'w+') as f:
        f.write('hello world\n')
    resp = requests.post(RENTER_ADDR + '/files', json={
        'sourcePath': os.path.abspath('files/share.txt'),
        'destPath': 'share.txt'
    })
    if resp.status_code != 201:
        print('error: {}'.format(resp.content.decode('utf-8')))
        sys.exit(1)

    fileId = json.loads(resp.content)['file']['id']
    
    print('sharing file')
    resp = requests.post(RENTER_ADDR + '/files/{}/permissions'.format(fileId), json={
        'userId': 'user1'
    })
    if resp.status_code != 201:
        print('error: {}'.format(resp.content.decode('utf-8')))
        sys.exit(1)

    print('PASS')

if __name__ == "__main__":
    main()
