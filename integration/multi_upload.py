
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

MIN_SIZE = 10 * 1024
MAX_SIZE = 5 * 1024 * 1024

files = [
    'school/WarAndPeace.txt',
    'school/CrimeAndPunishment.txt',
    'school/AliceInWonderland.txt',
    'work/VaxReport.doc',
    'work/analytics.xlsx',
    'work/talk.pdf',
    'pics/Seattle.png',
    'pics/NYC.jpeg',
    'pics/selfie.jpeg',
    'AliceInWonderland.txt',
    'screenshot.png',
    'draft.doc',
    'notes.txt',
    'amtoc.doc',
]

folders = [
    'school',
    'work',
    'pics',
]

def main():
    print(sys.argv[0])

    print('reserving space')
    reserve_space(1 << 30)

    print('creating folders')
    for folder in folders:
        os.mkdir('files/' + folder)

    print('creating files')
    for name in files:
        with open('files/' + name, 'w+') as f:
            size = random.randint(MIN_SIZE, MAX_SIZE)
            for i in range(size//4096):
                s = ''.join([string.ascii_uppercase[j % len(string.ascii_uppercase)]
                             for j in range(4096)])
                f.write(s)

    print('uploading folders')
    for folder in folders:
        upload_file(None, folder)

    print('uploading files')
    for name in files:
        upload_file(os.path.abspath('files/' + name), name)

    print('listing files')
    resp_obj = list_files()
    names = set([f['name'] for f in resp_obj['files']])
    for filename in files:
        assert(filename in names)
    for folder in folders:
        assert(folder in names)

    print('PASS')

if __name__ == "__main__":
    main()
