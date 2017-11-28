
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
    resp = upload_file(os.path.abspath('files/share.txt'), 'share.txt')
    file_id = resp['file']['id']
    
    print('sharing file')
    share_file(file_id, 'user1')

    print('PASS')

if __name__ == "__main__":
    main()
