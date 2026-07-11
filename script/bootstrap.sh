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
RUNTIME_LOG_ROOT=/opt/tiger/toutiao/log

if [ ! -d $RUNTIME_LOG_ROOT/app ]; then
    mkdir -p $RUNTIME_LOG_ROOT/app
fi

if [ ! -d $RUNTIME_LOG_ROOT/rpc ]; then
    mkdir -p $RUNTIME_LOG_ROOT/rpc
fi

export RUNTIME_SERVICE_PORT=$_BYTEFAAS_RUNTIME_PORT

SVC_NAME=bytedance.bytefaas.native_hertz_a2a-deep-agent-demo  # it's better to change to the real psm name
BinaryName=$SVC_NAME

export HERTZ_LOG_DIR=$RUNTIME_LOG_ROOT
export PSM=$SVC_NAME
CONF_DIR=$CURDIR/conf/

args="-psm=$SVC_NAME -conf-dir=$CONF_DIR -log-dir=$HERTZ_LOG_DIR"
if [ "X$PORT" != "X" ]; then
    args+=" -port=$PORT"
fi

echo "$CURDIR/bin/${BinaryName} $args"

exec $CURDIR/bin/${BinaryName} $args
