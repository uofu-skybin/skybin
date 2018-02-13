# Integration Tests

This directory contains useful scripts for running and testing SkyBin locally. 

To start a local metaserver, provider, and renter, run the `setup.sh` script in 
the background. Run `teardown.sh` to kill these services and remove related files.

Most other files in the directory are test scripts ending in `test.py`. To run one 
of these, use Python in the terminal, e.g.

`$ python single_upload_test.py`

