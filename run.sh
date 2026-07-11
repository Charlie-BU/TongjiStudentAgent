#!/bin/bash
set -ex
cd `dirname $0`

if [ -f .env ]; then
  export $(cat .env | sed 's/#.*//g' | xargs)
fi

# A special check for CLI users (run.sh should be located at the 'root' dir)
if [ -d "output" ]; then
    BOOTSTRAP_SCRIPT=./output/bootstrap.sh
else
    BOOTSTRAP_SCRIPT=./bootstrap.sh
fi

if [ -n "$TCE_PRIMARY_PORT" ]; then
    exec "$BOOTSTRAP_SCRIPT" "$1" "$TCE_PRIMARY_PORT" "${@:3}"
else
    exec "$BOOTSTRAP_SCRIPT" "$@"
fi