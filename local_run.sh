#!/usr/bin/env bash
set -ex

if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi

# If PORT0 is not set or empty, set it to 8080
if [[ -z "$PORT0" ]]; then
  export PORT0=8080
fi

exec ./output/bin/bytedance.bytefaas.native_hertz_a2a-deep-agent-demo -port=$PORT0
