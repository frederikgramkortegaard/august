#!/bin/bash

num=$1  # first argument

trap "kill 0" EXIT
    go run ./cmd/main.go -p2p "9360" &
for ((i=0; i<num-1; i++)); do
  go run ./cmd/main.go -p2p "937${i}" -seeds 0.0.0.0:9360 &
done
wait

