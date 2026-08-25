#!/bin/sh
index=0
while [ "$index" -lt 3000 ]; do
    printf 'x' >&2
    index=$((index + 1))
done
exit 1
