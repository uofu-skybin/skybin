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
../skybin metaserver &
sleep 1

echo "starting provider"
../skybin provider &

echo "starting renter"
../skybin renter &

wait
