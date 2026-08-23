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

Copy .env.example to .env and fill in at least:
  ARK_BASE_URL_CN=https://ark.cn-beijing.volces.com/api/v3
  ENDPOINT_ID=deepseek-v4-flash-ga-260731
  ENDPOINT_API_KEY=your-api-key

See .env.example and README.md for the full local configuration.
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

COZELOOP_ENABLED_VALUE="$(printf '%s' "${COZELOOP_ENABLED:-false}" | tr '[:upper:]' '[:lower:]')"
if [[ "${COZELOOP_ENABLED_VALUE}" == "1" || "${COZELOOP_ENABLED_VALUE}" == "true" || "${COZELOOP_ENABLED_VALUE}" == "yes" || "${COZELOOP_ENABLED_VALUE}" == "on" ]]; then
  if [[ -z "${COZELOOP_WORKSPACE_ID:-}" || -z "${COZELOOP_JWT_OAUTH_CLIENT_ID:-}" || -z "${COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID:-}" || -z "${COZELOOP_JWT_OAUTH_PRIVATE_KEY:-}" ]]; then
    echo "error: COZELOOP_WORKSPACE_ID, COZELOOP_JWT_OAUTH_CLIENT_ID, COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID, and COZELOOP_JWT_OAUTH_PRIVATE_KEY are required when COZELOOP_ENABLED=true" >&2
    exit 1
  fi
fi

KNOWLEDGE_ENABLED_VALUE="$(printf '%s' "${ARK_KNOWLEDGE_ENABLED:-false}" | tr '[:upper:]' '[:lower:]')"
if [[ "${KNOWLEDGE_ENABLED_VALUE}" == "1" || "${KNOWLEDGE_ENABLED_VALUE}" == "true" || "${KNOWLEDGE_ENABLED_VALUE}" == "yes" || "${KNOWLEDGE_ENABLED_VALUE}" == "on" ]]; then
  if [[ -z "${VOLC_API_KEY:-}" ]]; then
    echo "error: VOLC_API_KEY is required when ARK_KNOWLEDGE_ENABLED=true" >&2
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
