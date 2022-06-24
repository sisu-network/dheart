#!/usr/bin/env bash

CUR_DIR=$(pwd)
cd ../../../..
go build -o dheart
cp dheart $CUR_DIR/nodes/node0
cp dheart $CUR_DIR/nodes/node1

pkill dheart

# Run 2 dheart
cd $CUR_DIR/nodes/node0
./dheart &

cd $CUR_DIR/nodes/node1
./dheart &

# Run Sisu
# go run main-sisu.go &
