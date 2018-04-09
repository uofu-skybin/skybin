
"""
Renter bot for use in demonstration networks.
Runs continuously and performs random valid actions at a configurable frequency.

To use it, first set up and start a renter:

  $ skybin renter init
  $ skybin renter daemon &

Then you can run the renter bot with:

  $ python renter_bot.py

If the renter is set up at a non-default location (by passing the --homedir option
to 'renter init'), you can pass the renter's home folder to the bot with:

  $ python renter_bot.py --home_folder foo

Other options can be seen with:

  $ python renter_bot.py -h

"""

import argparse
import os
import json
import sys
import random
import test_framework
import time
import glob
import shutil
from pprint import pprint
from renter import RenterAPI

DEFAULT_HOME_DIR = os.path.join(os.path.expanduser('~'), '.skybin/renter')
DEFAULT_FILES_DIR = 'test_files_folder'
DEFAULT_OP_FREQ_SEC = 5.0
DEFAULT_MAX_USED_SPACE = 100 * 1024 * 1024
DEFAULT_INITIAL_RESERVED_SPACE = 1 * 1024 * 1024 * 1024
DEFAULT_MAX_RESERVED_SPACE = 5 * 1024 * 1024 * 1024
DEFAULT_MAX_FILES = 25
DEFAULT_NUM_TEST_FILES = 10
DEFAULT_MIN_FILE_SIZE = 1024
DEFAULT_MAX_FILE_SIZE = 10 * 1024 * 1024

MIN_RESERVATION_SIZE = 500 * 1024 * 1024
MAX_RESERVED_SPACE_SIZE = 3 * 1024 * 1024 * 1024
CHECKPOINT_FREQ_SEC = 5 * 60

ACTIONS = [
    'RESERVE',
    'UPLOAD',
    'DOWNLOAD',
    'REMOVE',
]

WEIGHTS = [
    0.05,
    0.2,
    0.6,
    0.15,
]

def run_step(renter, test_files, args):
    renter_info = renter.get_info()
    files = renter.list_files()

    action = random.choices(ACTIONS, weights=WEIGHTS)[0]

    if (action == 'REMOVE' or
        renter_info['totalFiles'] >= args.max_files or
        renter_info['usedStorage'] >= args.max_used_space) and len(files) > 0:

        f = random.choice(files)
        renter.remove_file(f['id'])
        return

    can_reserve_more_space = args.max_reserved_space - renter_info['reservedStorage'] > MIN_RESERVATION_SIZE
    if (action == 'RESERVE' or
        renter_info['freeStorage'] < args.max_file_size * 2) and can_reserve_more_space:
        max_amt = min(MAX_RESERVED_SPACE_SIZE, args.max_reserved_space - renter_info['reservedStorage'])
        amt = random.randint(MIN_RESERVATION_SIZE, max_amt)
        renter.reserve_space(amt)
        return

    if action == 'UPLOAD' or renter_info['totalFiles'] == 0:
        input_file = random.choice(test_files)
        renter.upload_file(input_file, test_framework.create_file_name())
        return

    file_to_download = random.choice(files)
    output_path = os.path.join(args.files_folder, 'download.txt')
    renter.download_file(file_to_download['id'], output_path)

def run_bot(renter, test_files, args):
    renter.reserve_space(args.initial_space)
    for _ in range(8):
        renter.upload_file(random.choice(test_files), test_framework.create_file_name())
    start_time = time.time()
    last_checkpoint = start_time
    num_ops = 0
    while True:
        try:
            run_step(renter, test_files, args)
        except Exception as err:
            print('EXCEPTION: ')
            print(err)
        num_ops += 1
        t = time.time()
        if t - last_checkpoint > CHECKPOINT_FREQ_SEC:
            renter_info = renter.get_info()
            print('Uptime: {} sec'.format(int(t - start_time)),
                  'Total operations:', num_ops,
                  'Storage reserved:', renter_info['reservedStorage'],
                  'Storage used:', renter_info['usedStorage'],
                  'Storage free:', renter_info['freeStorage'],
                  'Total files:', renter_info['totalFiles'],
                  'Total contracts:', renter_info['totalContracts'])
            last_checkpoint = t
        time.sleep(args.op_freq)


def main():
    parser = argparse.ArgumentParser(
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )
    parser.add_argument('--home_folder', type=str, default=DEFAULT_HOME_DIR,
                        help='Home folder of the renter to interact with')
    parser.add_argument('--files_folder', type=str, default=DEFAULT_FILES_DIR,
                        help='folder of possible files to upload. If the given folder already exists, ' +
                        'it will be used. Otherwise, it will be created.')
    parser.add_argument('--keep_files', action='store_true', default=False,
                        help='whether to keep test files after shutdown')
    parser.add_argument('--num_test_files', type=int, default=DEFAULT_NUM_TEST_FILES,
                        help='number of test files to create, if files_folder does not already exist')
    parser.add_argument('--min_file_size', type=int, default=DEFAULT_MIN_FILE_SIZE,
                        help='minimum size of test files to create, if files_folder does not already exist')
    parser.add_argument('--max_file_size', type=int, default=DEFAULT_MAX_FILE_SIZE,
                        help='maximum size of test files to create, if files_folder does not already exist')
    parser.add_argument('--op_freq', type=float, default=DEFAULT_OP_FREQ_SEC,
                        help='Frequency at which to perform actions on the network, in seconds')
    parser.add_argument('--max_used_space', type=int, default=DEFAULT_MAX_USED_SPACE,
                        help='maximum amount of space to use on the network.')
    parser.add_argument('--initial_space', type=int, default=DEFAULT_INITIAL_RESERVED_SPACE,
                        help='Space to initially reserve')
    parser.add_argument('--max_reserved_space', type=int, default=DEFAULT_MAX_RESERVED_SPACE,
                        help='maximum amount of space to reserve on the network.')
    parser.add_argument('--max_files', type=int, default=DEFAULT_MAX_FILES,
                        help='maximum number of files that can be uploaded at a time')
    args = parser.parse_args()

    if not os.path.isdir(args.home_folder):
        print('Error: no renter home folder found at ', args.home_folder)
        sys.exit(1)

    config_path = os.path.join(args.home_folder, 'config.json')
    with open(config_path) as f:
        config = json.loads(f.read())
    renter = RenterAPI('http://{}'.format(config['apiAddress']))

    try:
        renter.get_info()
    except Exception as e:
        print('Unable to ping renter at address ', config['apiAddress'], 'Are you sure it\'s running?')
        print('Error: ', e)
        sys.exit(1)

    if os.path.isdir(args.files_folder):
        test_files = glob.glob(args.files_folder + '/*')
        test_files = [os.path.abspath(name) for name in test_files]
    else:
        print('creating test files in ', args.files_folder)
        os.makedirs(args.files_folder)
        test_files = []
        for _ in range(args.num_test_files):
            name = test_framework.create_file_name()
            location = os.path.abspath(os.path.join(args.files_folder, name))
            size = random.randint(args.min_file_size, args.max_file_size)
            test_framework.create_test_file(location, size)
            test_files.append(location)

    print('starting renter bot')
    print('args:')
    pprint(vars(args))
    print('renter info:')
    pprint(renter.get_info())
    print('starting action loop')
    try:
        run_bot(renter, test_files, args)
    finally:
        if not args.keep_files:
            print('removing files folder')
            shutil.rmtree(args.files_folder)

if __name__ == '__main__':
    main()
