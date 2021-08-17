#!/bin/sh

# protoc proto/common/partyid.proto proto/ecdsa/keygen.proto -I. --go_out=types

function join_by { local IFS="$1"; shift; echo "$*"; }

CUR_DIR=$(pwd)
CUR_DIR_LENGTH=${#CUR_DIR}
echo $CUR_DIR_LENGTH

arr=()

for folder in $CUR_DIR/proto/**
do
  relative_path=${folder:$CUR_DIR_LENGTH+1}

  for file in $folder/*
  do
    f=$relative_path/$(basename $file)
    arr+=($f)
    # protoc $file -I. --go_out=types
  done
done

# Join all relative path
s=$(join_by " " "${arr[@]}")
echo $s

# Generate all proto files. These will go into folder github.com/sisu-network/dheart
protoc $s -I. --go_out=.

# Move all generated files into types folder at root
cp -r github.com/sisu-network/dheart/types .

# Delete github folder
rm -rf github.com