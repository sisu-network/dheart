#!/bin/sh

mkdir tmp

rsync -a --checksum --include "*.go" -exclude "tmp" . tmp
rm -rf tmp/tmp
rm -rf tmp/scripts

docker build . -t dheart