#!/usr/bin/env bash
set -ex

cd "$(dirname "$0")"

bash ./local_build.sh
exec bash ./local_run.sh
