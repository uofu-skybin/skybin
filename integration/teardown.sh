#! /usr/bin/env bash

echo "killing services and removing files"
ps -f | grep setup.sh | grep -v grep | awk '{print $2}' | xargs kill 
rm -rf repo*
rm -rf files
