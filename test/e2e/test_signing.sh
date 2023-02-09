#!/usr/bin/env bash

CUR_PATH=$(pwd)

run_test() {
  go run main.go -index 0 -seed $1 -n 2 &
  go run main.go -index 1 -seed $1 -n 2 &
  # go run main.go -index 2 -seed $1 -n 3 &

  echo "Waiting for all the jobs"

  for job in `jobs -p`
  do
    wait $job || {
      code="$?"
      exit "$code"
      break
    }
  done
}


cd './ecdsa/engine-keysign'

for i in {1..1}
do
  # do whatever on "$i" here
  run_test $i
  sleep 1
done

cd $CUR_PATH
