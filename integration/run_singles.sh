#!/usr/bin/env bash

echo "Running tests for single uploads"

run_test() {
    echo ""
    echo "num providers: $1"
    echo "file size: $2"
    echo "reservation size: $3"

    time python3 single_upload_test.py --num_providers=$1 \
            --file_size=$2 \
            --reservation_size=$3

    echo "done"
}

run_test 1 2 $((1024*1024*1024))
run_test 1 1024 $((1024*1024*1024))
run_test 1 $((10*1024*1024)) $((1024*1024*1024))
run_test 2 $((10*1024*1024)) $((2*1024*1024*1024))
run_test 3 $((10*1024*1024)) $((2*1024*1024*1024))
run_test 5 $((100*1024*1024)) $((5*1024*1024*1024))
