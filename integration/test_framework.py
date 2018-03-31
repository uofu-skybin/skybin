
"""
Framework and helpers for running integration tests.
"""

from renter import RenterAPI
import json
import os
import os.path
import sys
import random
import shutil
import string
import subprocess
import time
import sys

# Default location for skybin repos
DEFAULT_REPOS_DIR = os.path.abspath('./repos')

# Default location for test files
DEFAULT_TEST_FILE_DIR = os.path.abspath('./files')

# Whether test logging is enabled by default
LOG_ENABLED = True

# Whether test files are removed by default
REMOVE_TEST_FILES = True

SKYBIN_CMD = '../skybin'

def rand_port():
    return random.randint(32 * 1024, 64 * 1024)

def create_renter_alias():
    return 'renter_' + ''.join(str(random.randint(1, 9)) for _ in range(6))

def update_config_file(path, **kwargs):
    """Update a json config file"""
    with open(path, 'r') as config_file:
        config = json.loads(config_file.read())
    for k, v in kwargs.items():
        config[k] = v
    with open(path, 'w') as config_file:
        json.dump(config, config_file, indent=4, sort_keys=True)

def create_file_name():
    """Create a randomized name for a test file."""
    prefixes = ['homework', 'notes', 'photo', 'doc']
    unique_chars = ''.join(random.choice(string.ascii_lowercase) for _ in range(6))
    suffixes = ['.txt', '.jpg', '.doc']
    filename = ''.join([random.choice(prefixes), unique_chars, random.choice(suffixes)])
    return filename

def create_folder_name():
    """Create a randomized name for a test folder."""
    prefixes = ['folder', 'dir', 'things']
    unique_chars = ''.join(random.choice(string.ascii_lowercase) for _ in range(6))
    foldername = ''.join([random.choice(prefixes), unique_chars])
    return foldername

def create_test_file(location, size):
    """Create a test file

    Args:
      location: name for the file
      size: size of the file, in bytes

    Returns:
      the full name of the created test file
    """
    with open(location, 'wb+') as f:
        stride_len = 128 * 1024
        strides = size // stride_len
        for _ in range(strides):
            buf = os.urandom(stride_len)
            f.write(buf)
        rem = size % stride_len
        if rem != 0:
            buf = os.urandom(rem)
            f.write(buf)
    return location

def check_service_startup(process):
    """Check that a service process started without error."""
    if process.poll():
        rc = process.wait()
        _, stderr = process.communicate()
        raise ValueError(
            'service exited with error. code={}. stderr={}'.format(rc, stderr)
        )

def setup_db():
    """Run the setup db script."""
    process = subprocess.Popen(['mongo', 'setup_db.js'], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if process.wait() != 0:
        _, stderr = process.communicate()
        raise ValueError('setup_db failed. stderr={}'.format(stderr))

def teardown_db():
    """Run the teardown db script."""
    process = subprocess.Popen(['mongo', 'teardown_db.js'], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if process.wait() != 0:
        _, stderr = process.communicate()
        raise ValueError('teardown_db failed. stderr={}'.format(stderr))

class Service:
    """State container for a running skybin service

    Members:
      process: the running service process
      address: the service network address (host:port)
      homedir: directory for the service's files (or None)
    """

    def __init__(self, process, address, homedir=None):
        self.process = process
        self.address = address
        self.homedir = homedir

    def teardown(self, remove_files=True):
        self.process.kill()
        if self.homedir and remove_files:
            shutil.rmtree(self.homedir)

class RenterService(Service):
    """Specialization of Service to support the renter API"""

    def __init__(self, process, address, homedir=None):
        Service.__init__(self, process, address, homedir)
        base_api_url = 'http://{}'.format(address)
        self._api = RenterAPI(base_api_url)

    def get_info(self):
        return self._api.get_info()

    def reserve_space(self, amount):
        return self._api.reserve_space(amount)

    def upload_file(self, source, dest, should_overwrite=None):
        return self._api.upload_file(source, dest, should_overwrite=should_overwrite)

    def download_file(self, file_id, destination, version_num=None):
        return self._api.download_file(file_id, destination, version_num=version_num)

    def rename_file(self, file_id, name):
        return self._api.rename_file(file_id, name)

    def create_folder(self, name):
        return self._api.create_folder(name)

    def share_file(self, file_id, user_id):
        return self._api.share_file(file_id, user_id)

    def remove_file(self, file_id, version_num=None):
        return self._api.remove_file(file_id, version_num=version_num)

    def list_files(self):
        return self._api.list_files()

    def list_shared_files(self):
        return self._api.list_shared_files()

class TestContext:
    """Container for the services, test files, and helpers needed to run a test

    Members:
      metaserver: metaserver Service object
      providers: provider Service objects
      renter: renter Service object
      test_file_dir: location where test files are placed
      log_enabled: whether test logging is turned on
      remove_test_files: whether to remove test files on teardown
      teardown_db: whether to run the teardown_db script on teardown
    """

    def __init__(self, test_file_dir,
                 log_enabled=False,
                 remove_test_files=True,
                 teardown_db=True):
        self.metaserver = None
        self.providers = []
        self.renter = None
        self.additional_renters = []
        self.test_file_dir = test_file_dir
        self.log_enabled = log_enabled
        self.test_files = []
        self._remove_test_files = remove_test_files
        self._teardown_db = teardown_db

    def _remove_files(self):
        for filename in self.test_files:
            os.remove(filename)

    def create_test_file(self, size, parent_folder=None):
        """Create a test file. Returns the file's name."""
        parent_folder = parent_folder or self.test_file_dir
        file_name = create_file_name()
        file_path = '{}/{}'.format(parent_folder, file_name)
        create_test_file(file_path, size)
        self.test_files.append(file_path)
        return file_path

    def create_test_folder(self, parent_folder=None):
        """Create a test folder. Returns the folder's name."""
        parent_folder = parent_folder or self.test_file_dir
        folder_name = 'folder' + str(random.randint(1, 1000))
        folder_path = '{}/{}'.format(parent_folder, folder_name)
        os.makedirs(folder_path)
        return folder_path

    def create_output_path(self):
        """Get an output path that a file can be downloaded to."""
        filename = create_file_name()
        path = '{}/{}'.format(self.test_file_dir, filename)
        return path

    def create_folder_output_path(self):
        """Get an output path that a folder can be downloaded to."""
        foldername = create_folder_name()
        path = '{}/{}'.format(self.test_file_dir, foldername)
        return path

    def assert_true(self, condition, message=''):
        """If condition is not true, prints message and exits."""
        if not condition:
            print('FAIL:', message)
            self.teardown()
            sys.exit(1)

    def log(self, *args):
        """Print the given arguments."""
        if self.log_enabled:
            print(*args)

    def teardown(self):
        """Stop and clean up all services and files"""
        rm_files = self._remove_test_files
        if rm_files:
            self._remove_files()
        if self._teardown_db:
            teardown_db()
        if self.metaserver:
            self.metaserver.teardown(rm_files)
        for p in self.providers:
            p.teardown(rm_files)
        if self.renter:
            self.renter.teardown(rm_files)
        for r in self.additional_renters:
            r.teardown(rm_files)

def create_metaserver(api_addr=None, dashboard=False):
    """Create and start a new metaserver instance"""
    api_addr = api_addr or '127.0.0.1:{}'.format(rand_port())
    args = [SKYBIN_CMD, 'metaserver', '-addr', api_addr]
    if dashboard:
        args.append('-dash')
    process = subprocess.Popen(args, stderr=subprocess.PIPE)
    return Service(process=process, address=api_addr)

def init_renter(homedir, alias, metaserver_addr, api_addr):
    """Set up a skybin renter directory"""
    args = [SKYBIN_CMD, 'renter', 'init', '-homedir', homedir,
            '-alias', alias,
            '-meta-addr', metaserver_addr,
            '-api-addr', api_addr]
    process = subprocess.Popen(args, stderr=subprocess.PIPE)
    if process.wait() != 0:
        _, stderr = process.communicate()
        raise ValueError('renter init failed. stderr={}'.format(stderr))

def init_provider(homedir, metaserver_addr, public_api_addr, storage_space=50*1024*1024*1024):
    """Set up a skybin provider directory"""
    args = [SKYBIN_CMD, 'provider', 'init',
            '-homedir', homedir,
            '-meta-addr', metaserver_addr,
            '-public-api-addr', public_api_addr]
    if storage_space != None:
        args.append('-storage-space')
        args.append(str(storage_space))
    process = subprocess.Popen(args, stderr=subprocess.PIPE)
    if process.wait() != 0:
        _, stderr = process.communicate()
        raise ValueError('provider init failed. stderr={}'.format(stderr))

def create_renter(metaserver_addr, repo_dir, alias):
    """Create and start a new renter instance."""

    # Create repo
    homedir = '{}/renter{}'.format(repo_dir, random.randint(1, 1024))
    while os.path.exists(homedir):
        homedir = '{}/renter{}'.format(repo_dir, random.randint(1, 1024))

    api_addr = '127.0.0.1:{}'.format(rand_port())
    init_renter(
        homedir,
        alias=alias,
        metaserver_addr=metaserver_addr,
        api_addr=api_addr
    )

    # Start renter server
    env = os.environ.copy()
    env['SKYBIN_RENTER_HOME'] = homedir
    args = [SKYBIN_CMD, 'renter', 'daemon']
    process = subprocess.Popen(args, env=env, stderr=subprocess.PIPE)

    return RenterService(process=process, address=api_addr, homedir=homedir)

def create_provider(metaserver_addr, repo_dir,
                    api_addr=None,
                    storage_space=None):
    """Create and start a new provider instance."""

    homedir = '{}/provider{}'.format(repo_dir, random.randint(1, 1024))
    while os.path.exists(homedir):
        homedir = '{}/provider{}'.format(repo_dir, random.randint(1, 1024))

    api_addr = api_addr or '127.0.0.1:{}'.format(rand_port())
    init_provider(
        homedir,
        metaserver_addr=metaserver_addr,
        public_api_addr=api_addr,
        storage_space=storage_space,
    )

    # Start the provider daemon with no local API
    env = os.environ.copy()
    env['SKYBIN_PROVIDER_HOME'] = homedir
    args = [SKYBIN_CMD, 'provider', 'daemon', '--disable-local-api']
    process = subprocess.Popen(args, env=env, stderr=subprocess.PIPE)

    return Service(process=process, address=api_addr, homedir=homedir)

def setup_test(num_providers=1,
               repo_dir=DEFAULT_REPOS_DIR,
               test_file_dir=DEFAULT_TEST_FILE_DIR,
               log_enabled=LOG_ENABLED,
               remove_test_files=REMOVE_TEST_FILES,
               teardown_db=True,
               renter_alias=None,
               num_additional_renters=0):
    """Create a test context.

    Args:
      num_providers: number of provider services. Default 1
      repo_dir: directory to create test skybin repos in
      test_file_dir: directory to place test files in
      log_enabled: whether test logging is turned on
      remove_test_files: whether test files are removed on teardown
      teardown_db: whether to run the teardown_db script on teardown
      renter_alias: renter alias to use
    """
    if not os.path.exists(test_file_dir):
        os.makedirs(test_file_dir)
    if not os.path.exists(repo_dir):
        os.makedirs(repo_dir)
    if not renter_alias:
        renter_alias = create_renter_alias()
    ctxt = TestContext(test_file_dir=test_file_dir,
                       log_enabled=log_enabled,
                       remove_test_files=remove_test_files,
                       teardown_db=teardown_db)
    try:
        setup_db()
        ctxt.metaserver = create_metaserver()
        time.sleep(1.0)
        check_service_startup(ctxt.metaserver.process)
        for _ in range(num_providers):
            pvdr = create_provider(ctxt.metaserver.address, repo_dir=repo_dir)
            ctxt.providers.append(pvdr)
        ctxt.renter = create_renter(ctxt.metaserver.address, repo_dir=repo_dir,
                                    alias=renter_alias)
        for i in range(num_additional_renters):
            ctxt.additional_renters.append(
                create_renter(
                    ctxt.metaserver.address,
                    repo_dir=repo_dir,
                    alias=create_renter_alias(),
                )
            )
        time.sleep(1.0)
        for pvdr in ctxt.providers:
            check_service_startup(pvdr.process)
        check_service_startup(ctxt.renter.process)
    except Exception as err:
        ctxt.teardown()
        raise err
    return ctxt
