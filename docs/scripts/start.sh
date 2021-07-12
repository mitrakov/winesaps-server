#!/bin/bash

if [ $# -lt 1 ]; then echo "Usage $0 <program>"; exit 1; fi

if [ ! -f $1 ]; then echo "Program not found: $1"; exit 2; fi

while true; do
  ./$1
  printf "Process was terminated at $(date)\n" >> error.log
  sleep 1
done
