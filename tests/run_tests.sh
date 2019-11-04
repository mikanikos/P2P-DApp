#!/bin/bash

for i in {1..7}
do
   ./test_"$i"_ring.sh
   sleep 15
done
