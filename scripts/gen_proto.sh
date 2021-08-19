#!/bin/sh

function join_by { local IFS="$1"; shift; echo "$*"; }

CUR_DIR=$(pwd)
CUR_DIR_LENGTH=${#CUR_DIR}

# Gets all files in the proto folder and save them into an array.
arr=()
for folder in $CUR_DIR/proto/**
do
  relative_path=${folder:$CUR_DIR_LENGTH+1}

  for file in $folder/*
  do
    f=$relative_path/$(basename $file)
    arr+=($f)
  done
done

# Join all relative path
s=$(join_by " " "${arr[@]}")

# Generate all proto files. These will go into folder github.com/sisu-network/dheart
protoc $s -I. --go_out=.

# Move all generated files into types folder at root
cp -r github.com/sisu-network/dheart/types .

# Delete github folder
rm -rf github.com