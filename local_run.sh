#!/usr/bin/env bash
set -ex
export APP_ZONE=boe
export CONSUL_HTTP_HOST=common-consul-boe.bytedance.net
export CONSUL_HTTP_HOSTV6=fdbd:dc02:ff:1:2:225:130:44
export BOE_RESPECT_ENV=1
export TCE_ENV=prod
export ENABLE_REGION_LEVEL_CONFIG_FOR_DT=true
export MY_HOST_IPV6=::1
export BYTED_HOST_IPV6=::1
# If PORT0 is not set or empty, set it to 8080
if [[ -z "$PORT0" ]]; then
  export PORT0=8080
fi
export AGENT_DEBUG=true
export LOCAL_SERVER_DEBUG=true
export PSM=P.S.M
export HERTZ_CONF_DIR=./conf
export HERTZ_LOG_DIR=./log

exec ./output/bin/bytedance.bytefaas.native_hertz_a2a-deep-agent-demo -port=$PORT0
