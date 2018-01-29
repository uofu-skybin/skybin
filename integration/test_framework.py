
"""
Framework and helpers for running integration tests.
"""

from renter import RenterAPI
import json
import os
import random
import shutil
import string
import subprocess
import time

# Default location for skybin repos
DEFAULT_REPOS_DIR = './repos'

# Default location for test files
DEFAULT_TEST_FILE_DIR = './files'

# Whether test logging is enabled by default
LOG_ENABLED = True 

# Whether test files are removed by default
REMOVE_TEST_FILES = True

SKYBIN_CMD = '../skybin'

def rand_port():
    return random.randint(32 * 1024, 64 * 1024)

def update_config_file(path, **kwargs):
    """Update a json config file"""
    with open(path, 'r') as config_file:
        config = json.loads(config_file.read())
    for k, v in kwargs.items():
        config[k] = v
    with open(path, 'w') as config_file:
        json.dump(config, config_file, indent=4, sort_keys=True)

def init_repo(homedir):
    """Create a skybin repo"""
    args = [SKYBIN_CMD, 'init', '-home', homedir]
    process = subprocess.Popen(args, stderr=subprocess.PIPE)
    if process.wait() != 0:
        _, stderr = process.communicate()
        raise ValueError('skybin init failed. stderr={}'.format(stderr))

def create_file_name():
    """Create a randomized name for a test file."""
    prefixes = ['homework', 'notes', 'photo', ]
    unique_chars = ''.join(random.choice(string.ascii_lowercase) for _ in range(4))
    suffixes = ['.txt', '.jpg', '.doc']
    filename = ''.join([random.choice(prefixes), unique_chars, random.choice(suffixes)])
    return filename

def create_test_file(file_dir, size):
    """Create a test file

    Args:
      file_dir: directory to place file
      size: size of the file, in bytes

    Returns:
      the full name of the created test file
    """
    filename = create_file_name()
    filepath = '{}/{}'.format(file_dir, filename)
    with open(filepath, 'wb+') as f:
        stride_len = 128 * 1024
        strides = size // stride_len
        for _ in range(strides):
            buf = os.urandom(stride_len)
            f.write(buf)
        rem = size % stride_len
        if rem != 0:
            buf = os.urandom(rem)
            f.write(buf)
    return filepath

def check_service_startup(process):
    """Check that a service started without error."""
    # Wait for startup
    time.sleep(0.5)
    if process.poll():
        rc = process.wait()
        _, stderr = process.communicate()
        raise ValueError(
            'service exited with error. code={}. stderr={}'.format(rc, stderr)
        )

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

    def reserve_space(self, amount):
        return self._api.reserve_space(amount)

    def upload_file(self, source, dest):
        return self._api.upload_file(source, dest)

    def download_file(self, file_id, destination):
        return self._api.download_file(file_id, destination)

    def share_file(self, file_id, user_id):
        return self._api.share_file(file_id, user_id)

    def remove_file(self, file_id):
        return self._api.remove_file(file_id)

    def list_files(self):
        return self._api.list_files()

class TestContext:
    """Container for the services, test files, and helpers needed to run a test

    Members:
      metaserver: metaserver Service object
      providers: provider Service objects
      renter: renter Service object
      test_file_dir: location where test files are placed
      log_enabled: whether test logging is turned on
      remove_test_files: whether to remove test files on teardown
    """

    def __init__(self, test_file_dir,
                 log_enabled=False,
                 remove_test_files=True):
        self.metaserver = None
        self.providers = []
        self.renter = None
        self.test_file_dir = test_file_dir
        self.log_enabled = log_enabled
        self._test_files = []
        self._remove_test_files = remove_test_files

    def _remove_files(self):
        for filename in self._test_files:
            os.remove(filename)

    def create_test_file(self, size):
        """Create a test file. Returns the file's name."""
        file_name = create_test_file(self.test_file_dir, size)
        self._test_files.append(file_name)
        return file_name

    def create_output_path(self):
        """Get an output path that a file can be downloaded to."""
        filename = create_file_name()
        path = '{}/{}'.format(self.test_file_dir, filename)
        return path

    def assert_true(self, condition, message=''):
        """If condition is not true, prints message and exits."""
        if not condition:
            print('FAIL:', message)
            self.teardown()
            os.exit(1)

    def log(self, *args):
        """Print the given arguments."""
        if self.log_enabled:
            print(*args)

    def teardown(self):
        """Stop and clean up all services and files"""
        rm_files = self._remove_test_files
        if rm_files:
            self._remove_files()
        if self.metaserver:
            self.metaserver.teardown(rm_files)
        for p in self.providers:
            p.teardown(rm_files)
        if self.renter:
            self.renter.teardown(rm_files)

def create_metaserver():
    """Create and start a new metaserver instance"""

    port = rand_port()
    address = '127.0.0.1:{}'.format(port)
    args = [SKYBIN_CMD, 'metaserver', '-addr', address]
    process = subprocess.Popen(args, stderr=subprocess.PIPE)
    check_service_startup(process)
    return Service(process=process, address=address)

def create_renter(metaserver_addr, repo_dir):
    """Create and start a new renter instance."""

    # Create repo
    homedir = '{}/renter{}'.format(repo_dir, random.randint(1, 1024))
    init_repo(homedir)

    # Update default renter config
    api_address = '127.0.0.1:{}'.format(rand_port())
    config_path = '{}/renter/config.json'.format(homedir)
    update_config_file(config_path,
        apiAddress=api_address,
        metaServerAddress=metaserver_addr,
    )

    # Start renter server
    env = os.environ.copy()
    env['SKYBIN_HOME'] = homedir
    args = [SKYBIN_CMD, 'renter']
    process = subprocess.Popen(args, env=env, stderr=subprocess.PIPE)

    check_service_startup(process)

    return RenterService(process=process, address=api_address, homedir=homedir)

def create_provider(metaserver_addr, repo_dir):
    """Create and start a new provider instance."""

    homedir = '{}/provider{}'.format(repo_dir, random.randint(1, 1024))
    init_repo(homedir)

    api_address = '127.0.0.1:{}'.format(rand_port())
    config_path = '{}/provider/config.json'.format(homedir)
    update_config_file(
        config_path,
        apiAddress=api_address,
        metaServerAddress=metaserver_addr,
    )

    env = os.environ.copy()
    env['SKYBIN_HOME'] = homedir
    args = [SKYBIN_CMD, 'provider']
    process = subprocess.Popen(args, env=env, stderr=subprocess.PIPE)

    check_service_startup(process)

    return Service(process=process, address=api_address, homedir=homedir)

def setup_test(num_providers=1,
               repo_dir=DEFAULT_REPOS_DIR,
               test_file_dir=DEFAULT_TEST_FILE_DIR,
               log_enabled=LOG_ENABLED,
               remove_test_files=REMOVE_TEST_FILES):
    """Create a test context.

    Args:
      num_providers: number of provider services. Default 1
      repo_dir: directory to create test skybin repos in
      test_file_dir: directory to place test files in
      log_enabled: whether test logging is turned on
      remove_test_files: whether test files are removed on teardown
    """
    if not os.path.exists(test_file_dir):
        os.makedirs(test_file_dir)
    if not os.path.exists(repo_dir):
        os.makedirs(repo_dir)
    ctxt = TestContext(test_file_dir=test_file_dir,
                       log_enabled=log_enabled,
                       remove_test_files=remove_test_files)
    try:
        ctxt.metaserver = create_metaserver()
        for _ in range(num_providers):
            pvdr = create_provider(ctxt.metaserver.address, repo_dir=repo_dir)
            ctxt.providers.append(pvdr)
        ctxt.renter = create_renter(ctxt.metaserver.address, repo_dir=repo_dir)
    except Exception as err:
        ctxt.teardown()
        raise err
    return ctxt
