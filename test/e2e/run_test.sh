#!/usr/bin/env bash


# TODO: Fix flakiness of the p2p lib. Sometimes, a node cannot connect to its peers. Because of this
# flakiness, we don't want to include this test scripts into our CI flow yet.

# FOLDERS=('core-heart/keygen' 'core-heart/presign' 'engine-keygen' 'p2p-network')
FOLDERS=('engine/ecdsa-keysign')


CUR_PATH=$(pwd)

run_test() {
  echo "Running test in folder" $1
  cd $1

  n=4
  for i in $(seq $n); do
    index=$(($i-1))
    PROJECT_ID=$PROJECT_ID go run main.go -n $n -index $index &
  done

  echo "Waiting for all the jobs"

  for job in `jobs -p`
  do
    wait $job || {
      code="$?"
      ([[ $code = "127" ]] && exit 0 || exit "$code")
      break
    }
  done
}

for folder in "${FOLDERS[@]}"
do
  cd $CUR_PATH
  # do whatever on "$i" here
  run_test $folder
  sleep 2
done
