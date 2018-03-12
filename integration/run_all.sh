#! /usr/bin/env bash

# Tests with a single provider
echo "Running tests with 1 provider"
python3.6 single_upload_test.py 
python3.6 multi_upload_test.py 
python3.6 rename_test.py 
python3.6 share_file_test.py
python3.6 rm_file_test.py
python3.6 file_recovery_test.py
python3.6 folder_download_test.py
python3.6 folder_upload_test.py
python3.6 concurrent_test.py
python3.6 version_test.py

# Tests with 10 providers
echo "Repeating tests with 10 providers"
python3.6 single_upload_test.py --num_providers 10
python3.6 multi_upload_test.py --num_providers 10
python3.6 share_file_test.py --num_providers 10
python3.6 rm_file_test.py --num_providers 10
python3.6 folder_download_test.py --num_providers 10
python3.6 folder_upload_test.py --num_providers 10
python3.6 concurrent_test.py --num_providers 10
python3.6 version_test.py --num_providers 10