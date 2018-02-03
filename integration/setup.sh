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

echo "setting up sample skybin repo"
$SKYBIN_CMD init -home $REPO_DIR
export SKYBIN_HOME=$PWD/$REPO_DIR

echo "setting up database"
mongo setup_db.js

echo "creating directory for test files"
mkdir $TEST_FILE_DIR

echo "starting services"

echo "starting metaserver"
$SKYBIN_CMD metaserver &
sleep 1

echo "starting provider"
$SKYBIN_CMD provider &

echo "starting renter"
$SKYBIN_CMD renter -alias test &

wait
