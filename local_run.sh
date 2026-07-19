#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${ROOT_DIR}/.env"

usage() {
  cat <<'EOF'
Usage: ./local_run.sh [port]

Environment:
  SKIP_GO_MOD_DOWNLOAD=1  Skip "go mod download" before startup.

Examples:
  ./local_run.sh
  ./local_run.sh 8081
  PORT0=8082 ./local_run.sh
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is not installed or not in PATH" >&2
  exit 1
fi

cd "${ROOT_DIR}"

if [[ ! -f "${ENV_FILE}" ]]; then
  cat >&2 <<'EOF'
error: .env not found in repository root.

Create .env with at least:
  ARK_BASE_URL_CN=https://your-model-endpoint
  ENDPOINT_ID=your-endpoint-id
  ENDPOINT_API_KEY=your-api-key

See README.md for the full local configuration example.
EOF
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

if [[ -z "${ENDPOINT_ID:-}" ]]; then
  echo "error: ENDPOINT_ID is required" >&2
  exit 1
fi

if [[ -z "${ENDPOINT_API_KEY:-}" ]]; then
  echo "error: ENDPOINT_API_KEY is required" >&2
  exit 1
fi

if [[ -z "${ARK_BASE_URL:-}" && -z "${ARK_BASE_URL_CN:-}" ]]; then
  echo "error: ARK_BASE_URL or ARK_BASE_URL_CN is required" >&2
  exit 1
fi

FORNAX_ENABLED_VALUE="$(printf '%s' "${FORNAX_ENABLED:-false}" | tr '[:upper:]' '[:lower:]')"
if [[ "${FORNAX_ENABLED_VALUE}" == "1" || "${FORNAX_ENABLED_VALUE}" == "true" || "${FORNAX_ENABLED_VALUE}" == "yes" || "${FORNAX_ENABLED_VALUE}" == "on" ]]; then
  if [[ -z "${FORNAX_AK:-}" || -z "${FORNAX_SK:-}" ]]; then
    echo "error: FORNAX_AK and FORNAX_SK are required when FORNAX_ENABLED=true" >&2
    exit 1
  fi
fi

KNOWLEDGE_ENABLED_VALUE="$(printf '%s' "${ARK_KNOWLEDGE_ENABLED:-false}" | tr '[:upper:]' '[:lower:]')"
if [[ "${KNOWLEDGE_ENABLED_VALUE}" == "1" || "${KNOWLEDGE_ENABLED_VALUE}" == "true" || "${KNOWLEDGE_ENABLED_VALUE}" == "yes" || "${KNOWLEDGE_ENABLED_VALUE}" == "on" ]]; then
  if [[ -z "${ARK_AK:-}" || -z "${ARK_SK:-}" ]]; then
    echo "error: ARK_AK and ARK_SK are required when ARK_KNOWLEDGE_ENABLED=true" >&2
    exit 1
  fi
  if [[ -z "${ARK_KNOWLEDGE_COLLECTION:-}" && -z "${ARK_KNOWLEDGE_RESOURCE_ID:-}" ]]; then
    echo "error: ARK_KNOWLEDGE_COLLECTION or ARK_KNOWLEDGE_RESOURCE_ID is required when ARK_KNOWLEDGE_ENABLED=true" >&2
    exit 1
  fi
fi

PORT_VALUE="${1:-${PORT0:-8080}}"
export PORT0="${PORT_VALUE}"

if [[ "${SKIP_GO_MOD_DOWNLOAD:-0}" != "1" ]]; then
  echo "==> Downloading Go modules"
  go mod download
fi

echo "==> Starting TongjiStudent on port ${PORT0}"
exec go run . -port="${PORT0}"
