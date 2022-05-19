#!/usr/bin/env bash

CUR_PATH=$(pwd)

run_test() {
  go run main.go -index 0 -seed $1 -n 3 &
  go run main.go -index 1 -seed $1 -n 3 &
  go run main.go -index 2 -seed $1 -n 3 &

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


cd 'engine-keysign'

for i in {1..2}
do
  # do whatever on "$i" here
  run_test $i
  sleep 1
done

cd $CUR_PATH
