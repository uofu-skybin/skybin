#! /usr/bin/env bash

set -e

trap "exit" INT TERM
trap "kill 0" EXIT

echo "building skybin"
cd .. && go build
cd -

echo "setting up sample skybin repo"
../skybin init -home repo
export SKYBIN_HOME=$PWD/repo

echo "creating directory for test files"
mkdir files

echo "starting services"

echo "starting metaserver"
../skybin metaserver > /dev/null &
sleep 1

echo "starting provider"
../skybin provider > /dev/null &

echo "starting renter"
../skybin renter > /dev/null &

wait
