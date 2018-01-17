#! /usr/bin/env bash

set -e

trap "exit" INT TERM
trap "kill 0" EXIT

REPO_DIR="./repo"
TEST_FILE_DIR="./files"

echo "building skybin"
cd .. && go build
cd -

echo "setting up sample skybin repo"
../skybin init -home $REPO_DIR
export SKYBIN_HOME=$PWD/$REPO_DIR

echo "creating directory for test files"
mkdir $TEST_FILE_DIR

echo "starting services"

echo "starting metaserver"
../skybin metaserver &
sleep 1

echo "starting provider"
../skybin provider &

echo "starting renter"
../skybin renter &

wait
