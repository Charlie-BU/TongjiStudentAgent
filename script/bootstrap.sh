#!/bin/bash
CURDIR=$(cd $(dirname $0); pwd)
if [ "X$1" != "X" ]; then
    RUNTIME_ROOT=$1
else
    RUNTIME_ROOT=${CURDIR}
fi

if [ "X$RUNTIME_ROOT" == "X" ]; then
    echo "There is no RUNTIME_ROOT support."
    echo "Usage: ./bootstrap.sh $RUNTIME_ROOT"
    exit -1
fi

PORT=$2

RUNTIME_CONF_ROOT=/tmp/conf
SVC_NAME=bytedance.bytefaas.native_hertz_a2a-deep-agent-demo  # it's better to change to the real psm name
BinaryName=$SVC_NAME

args=""
if [ "X$PORT" != "X" ]; then
    args="-port=$PORT"
fi

echo "$CURDIR/bin/${BinaryName} $args"

exec $CURDIR/bin/${BinaryName} $args
