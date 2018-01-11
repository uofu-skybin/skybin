#! /usr/bin/env bash

set -e

trap "exit" INT TERM
trap "kill 0" EXIT

echo "building skybin"
cd .. && go build
cd -


echo "starting services"

echo "starting metaserver"
../skybin metaserver &
sleep 1

echo "creating directory for test files"
mkdir files

for (( i=1; i<=13; i++))
	do
		echo "setting up provider $i"
        ../skybin init -home repo/provider$i
        export SKYBIN_HOME=$PWD/repo/provider$i
        port=$((i + 9000))
        ../skybin provider -addr :$port &
	done

# echo "starting provider"
# ../skybin provider &

echo "setting up renter"
../skybin init -home repo/renter
export SKYBIN_HOME=$PWD/repo/renter

echo "starting renter"
../skybin renter &

wait
