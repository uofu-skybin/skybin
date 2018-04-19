# Integration Tests

This directory contains scripts for running and testing SkyBin locally.

## Overview

- `setup.sh` and `teardown.sh` setup and teardown a local test network with a
   single renter and provider. This is useful for running SkyBin locally during
   development.
- `test_framework.py` contains common code used to create and run integration tests.
- `*_test.py` files contain single integration tests.
- `test_net.py` runs a configurable local SkyBin network and performs random
   actions on the network (uploads, downloads, etc.).

## Running the SkyBin services locally

To start a local metaserver, provider, and renter, run the `setup.sh` script in 
the background. You can then reserve storage, store files, and perform other actions
via the skybin binary. Use `teardown.sh` to clean up the network when finished.

Example:

```
$ ./setup.sh &

# Reserve one GB of space.
$ skybin reserve 1gb

# Upload a file.
$ head -c 10000 /dev/urandom > sample.txt
$ skybin upload sample.txt

# Check that the file exists.
$ skybin list

# Download the file to "sample2.txt".
$ skybin download sample.txt sample2.txt

# Remove the file.
$ skybin rm sample.txt

# Shut down the test network.
$ ./teardown.sh
```

## Running a single test

Run a single integration test with Python. Example:

```
$ python3 single_upload_test.py
```

Tests print "ok" upon passing and exit with an exception
if failure occurs.

