
# Check that a file can be uploaded and later removed.

from constants import *
from helpers import *
import sys
import os

def main():
    print(sys.argv[0])

    print('reserving space')
    reserve_space(1 << 20)

    print('creating file')
    filename = "rmsample1.txt"
    with open('files/' + filename, 'w+') as f:
        s = '1234567890'
        for _ in range(1 << 17):
            f.write(s)
            f.write('\n')

    print('uploading file')
    resp = upload_file(os.path.abspath('files/' + filename), filename)

    print('removing file')
    file_id = resp['file']['id']
    remove_file(file_id)

    print('checking listed files')
    resp = list_files()
    if any([f for f in resp['files'] if f['id'] == file_id]):
        print('error: file still listed')
        print('files: ', resp['files'])
        sys.exit(1)

    print('PASS')

if __name__ == "__main__":
    main()
