#! /usr/bin/env bash

set -e

trap "exit" INT TERM
trap "kill 0" EXIT

SKYBIN_CMD="../skybin"
REPO_DIR="./repo"
TEST_FILE_DIR="./files"
RENTER_ALIAS="test"

echo "building skybin"
cd .. && go build
cd -

echo "setting up database"
mongo setup_db.js

echo "starting metaserver"
$SKYBIN_CMD metaserver -dash &
sleep 1

echo "setting up sample skybin repo"
$SKYBIN_CMD renter init --alias test_renter --homedir $REPO_DIR/renter
$SKYBIN_CMD provider init --homedir $REPO_DIR/provider
export SKYBIN_HOME=$PWD/$REPO_DIR

echo "creating directory for test files"
mkdir $TEST_FILE_DIR

echo "starting provider"
$SKYBIN_CMD provider daemon &

echo "starting renter"
$SKYBIN_CMD renter daemon &

wait
