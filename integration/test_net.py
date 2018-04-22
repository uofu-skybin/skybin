
"""
Runs a test network which performs random uploads/downloads among
a set of renters and providers.

Important:

 - This will not set up or tear down the metaserver database. You
   must do that separately.
 - However, this _will_ run a metaserver instance. If you're running
   this on a cloud VM and want to make the metaserver and renters publicly
   available, set the IP address option to the VM's IP and the meta-port option
   to the port you want the metaserver to listen on.
 - If you want to keep the renter/provider repos and test files this creates
   after killing the test network (perhaps for debugging), set the --keep_files option.
 - You run the test net with existing providers and renters by setting the --files_dir
   and --repo_dir options to folders which were previously created. If you run
   the test net with default files and repo dirs, then merely starting and stopping
   the script without manually deleting these folders will do this.
 - This creates a log file, test_net.log, in the current directory which you can
   view to see which operations are being run and by which renters.

"""

import argparse
import os
import glob
import time
import random
import shutil
import sys
import test_framework

# We configure providers with lots of storage space to avoid
# running out.
PROVIDER_STORAGE_SPACE = 100 * 1024 * 1024 * 1024

# No single renter should reserve more than this much space.
MAX_RESERVED_SPACE = 10 * 1024 * 1024 * 1024

# Maximum amount of storage a renter should use in the network.
# This is set fairly low to keep disk usage reasonable.
MAX_USED_SPACE = 512 * 1024 * 1024

# The test network should sleep at least this long between
# each upload/download operation
MIN_SLEEP_SEC = 0.5
MAX_SLEEP_SEC = 5.0

# The test net runner should print out a checkpoint summary this often.
CHECKPOINT_FREQ_SEC = 5 * 60

LOG_FILE = 'test_net.log'

def create_dest_path(files):
    """Creates a path to upload a file to which does not already
    exist in the given list of skybin files.
    """
    names = set(f['name'] for f in files)
    while True:
        path = test_framework.create_file_name()
        if path in names:
            continue
        return path

class TestNet:

    def __init__(self):

        # List of full file names available for uploads
        self.files = []

        # Service objects
        self.metaserver = None
        self.renters = []
        self.providers = []

        # Command-line options
        self.options = None

        self.log_file = None

    def _pick_test_file(self):
        """Chooses a random test file."""
        return random.choice(self.files)

    def _create_download_path(self):
        """Returns a location to download a file to."""

        # We download all files to the same place.
        return '{}/{}'.format(self.options.files_dir, 'foo.txt')

    def log_op(self, renter_info, op):
        self.log_file.write('{}: renter alias={} op={}\n'.format(
            time.asctime(), renter_info['alias'], op))
        self.log_file.flush()

    def run_step(self):
        renter = random.choice(self.renters)
        renter_info = renter.get_info()
        files = renter.list_files()

        # If the renter doesn't have any storage yet,
        # reserve a large initial chunk in order to
        # create contracts with a good fraction of the providers.
        if renter_info['reservedStorage'] == 0:
            self.log_op(renter_info, 'reserving initial storage')
            for _ in range(10):
                renter.reserve_space(512*1024*1024)
            return

        # If the renter has no files, upload a bunch of small ones to get started.
        if renter_info['totalFiles'] == 0:
            self.log_op(renter_info, 'uploading first files')
            input_file = self.files[0]
            for _ in range(20):
                dest_path = create_dest_path(files)
                files.append(renter.upload_file(input_file, dest_path))
            return

        if renter_info['usedStorage'] >= MAX_USED_SPACE:
            self.log_op(renter_info, 'removing file (used storage exceeds max used space)')
            f = random.choice(files)
            renter.remove_file(f['id'])
            return

        if renter_info['freeStorage'] <  self.options.max_file_size*2:
            self.log_op(renter_info, 'removing file (free storage insufficient for upload)')
            f = random.choice(files)
            renter.remove_file(f['id'])
            return

        # At this point, the renter should:
        #   - Have 1 or more files
        #   - Have enough free space to upload any test file
        r = random.random()
        if r < 0.4 or len(files) < 10:
            self.log_op(renter_info, 'uploading file')
            input_file = self._pick_test_file()
            dest_path = create_dest_path(files)
            renter.upload_file(input_file, dest_path)
            return

        if r < 0.6:
            self.log_op(renter_info, 'removing file')
            f = random.choice(files)
            renter.remove_file(f['id'])
            return

        if r < 0.9:
            self.log_op(renter_info, 'downloading file')
            file_to_download = random.choice(files)
            download_path = self._create_download_path()
            renter.download_file(file_to_download['id'], download_path)
            return

        if renter_info['reservedStorage'] >= MAX_RESERVED_SPACE:
            # No-op!
            return

        space = random.randint(50*1024*1024, 250*1024*1024)
        self.log_op(renter_info, 'reserving {} bytes'.format(space))
        renter.reserve_space(space)

    def run(self):
        start_time = time.time()
        last_checkpoint_time = start_time
        total_steps = 0
        while True:
            try:
                self.run_step()
            except Exception as err:
                print('EXCEPTION')
                print(err)
                self.log_file.write('Exception while running step: {}\n'.format(str(err)))
            sleep_time = random.uniform(MIN_SLEEP_SEC, MAX_SLEEP_SEC)
            time.sleep(sleep_time)
            total_steps += 1
            t = time.time()
            if t - last_checkpoint_time > CHECKPOINT_FREQ_SEC:
                print('test net')
                print('Up Time: {:.2f} sec'.format(t - start_time))
                print('Total Operations: {}'.format(total_steps))
                print('')
                last_checkpoint_time = t

    def teardown(self):
        """Kills all services and cleans up test directories."""
        rm_files = not self.options.keep_files
        for r in self.renters:
            r.teardown(rm_files)
        for p in self.providers:
            p.teardown(rm_files)
        self.metaserver.teardown(rm_files)
        if rm_files:
            shutil.rmtree(self.options.files_dir)
        if rm_files:
            shutil.rmtree(self.options.repo_dir)

def create_files(options):
    os.makedirs(options.files_dir)
    files = []
    step_size = int((options.max_file_size - options.min_file_size) / options.num_files)
    for i in range(options.num_files):
        file_name = test_framework.create_file_name()
        full_path = '{}/{}'.format(options.files_dir, file_name)
        min_size = options.min_file_size + i * step_size
        max_size = min_size + step_size
        file_size = random.randint(min_size, max_size)
        test_framework.create_test_file(full_path, file_size)
        files.append(full_path)
    return files

def setup_test_net(options):
    net = TestNet()
    net.options = options
    net.log_file = open(LOG_FILE, 'w+')

    if os.path.exists(options.files_dir):
        print('using existing folders and files found in {}'.format(options.files_dir))
        net.files = glob.glob('{}/*'.format(options.files_dir))
    else:
        print('creating folders and files')
        net.files = create_files(options)

    meta_addr = '{}:{}'.format(options.ip_addr, options.meta_port)
    print('starting metaserver at {}'.format(meta_addr))
    net.metaserver = test_framework.start_metaserver(api_addr=meta_addr, dashboard=True)

    # Wait for metaserver to start before starting other services
    time.sleep(1.0)
    test_framework.check_service_startup(net.metaserver.process)

    if os.path.exists(options.repo_dir):
        print('starting existing renters and providers found in {}'.format(options.repo_dir))
        renter_homedirs = glob.glob('{}/renter*'.format(options.repo_dir))
        provider_homedirs = glob.glob('{}/provider*'.format(options.repo_dir))
        for renter_homedir in renter_homedirs:
            net.renters.append(test_framework.start_renter(renter_homedir))
        for provider_homedir in provider_homedirs:
            net.providers.append(test_framework.start_provider(provider_homedir))
    else:
        print('creating renters and providers in {}'.format(options.repo_dir))
        os.makedirs(options.repo_dir)
        for _ in range(options.num_renters):
            renter = test_framework.create_renter(
                metaserver_addr=meta_addr,
                repo_dir=options.repo_dir,
                alias=test_framework.create_renter_alias(),
            )
            net.renters.append(renter)
        for _ in range(options.num_providers):
            api_addr = '{}:{}'.format(options.ip_addr, test_framework.rand_port())
            provider = test_framework.create_provider(
                metaserver_addr=meta_addr,
                repo_dir=options.repo_dir,
                api_addr=api_addr,
                storage_space=PROVIDER_STORAGE_SPACE,
                min_storage_rate=options.min_storage_rate,
                max_storage_rate=options.max_storage_rate,
                pricing_policy=options.pricing_policy
            )
            net.providers.append(provider)

    print('checking server startups')
    time.sleep(1.0)
    for renter in net.renters:
        test_framework.check_service_startup(renter.process)
    for provider in net.providers:
        test_framework.check_service_startup(provider.process)

    print('done')
    return net

def main():
    parser = argparse.ArgumentParser(
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )
    parser.add_argument('--num_providers', type=int, default=3,
                        help='number of providers to run')
    parser.add_argument('--num_renters', type=int, default=1,
                        help='number of renters to run')
    parser.add_argument('--files_dir', type=str, default='files',
                        help='directory to place test files')
    parser.add_argument('--repo_dir', type=str, default='repos',
                        help='directory to place renter/provider repos')
    parser.add_argument('--ip_addr', type=str, default='127.0.0.1',
                        help='IP address to use for the public provider/metaserver APIs')
    parser.add_argument('--meta_port', type=int, default=8001,
                        help='port to run the public metaserver API at')
    parser.add_argument('--keep_files', action='store_true', default=False,
                        help='whether to keep test files and repo directories when shutting down')
    parser.add_argument('--num_files', type=int, default=10,
                        help='number of test files to create for uploads')
    parser.add_argument('--min_file_size', type=int, default=10000,
                        help='minimum test file size')
    parser.add_argument('--max_file_size', type=int, default=10000000,
                        help='maximum test file size')
    parser.add_argument('--min_storage_rate', type=int, default=None,
                        help='minimum storage rate to set for all providers')
    parser.add_argument('--max_storage_rate', type=int, default=None,
                        help='maximum storage rate to set for all providers')
    parser.add_argument('--pricing_policy', type=str, default=None,
                        help='pricing policy to set for all providers')

    args = parser.parse_args()

    assert args.min_file_size < args.max_file_size, 'min_file_size must be less than max_file_size'

    # Be sure that the files_dir and repo_dir args use absolute paths.
    args.files_dir = os.path.abspath(args.files_dir)
    args.repo_dir = os.path.abspath(args.repo_dir)

    net = setup_test_net(args)
    try:
        print('starting test net operations')
        print('see {} for operation log'.format(LOG_FILE))
        net.run()
    finally:
        net.teardown()

if __name__ == '__main__':
    main()
