#!/bin/sh
# span_invoke_start.sh — Record span start and send a skill.invoke start event to MCS.
# Reads skill metadata from meta.json (no args required).
# Requires: sh, curl, od, tr, grep, cut, sed, date. No python, no jq, no openssl.
#
# Environment variables (injected by agent runtime when available):
#   AGENTBUDDY_SESSION_ID   — stable session identifier
#   AGENTBUDDY_USERNAME     — authenticated username (auto-resolved if not set)
#   AGENTBUDDY_AGENT_NAME   — agent name
#   AGENTBUDDY_AGENT_MODEL  — LLM model identifier
#   AGENTBUDDY_DEBUG        — set to "1" for diagnostic output on stderr

set +e

SCRIPT_DIR="$(dirname "$0")"
# shellcheck source=telemetry_lib.sh
. "$SCRIPT_DIR/telemetry_lib.sh"
# shellcheck source=resolve_username.sh
. "$SCRIPT_DIR/resolve_username.sh"

_dbg "span_invoke_start.sh begin"

# ── Read meta.json ────────────────────────────────────────────────────
# shellcheck disable=SC2034
META="$SCRIPT_DIR/meta.json"
SKILL_NAME=$(_jread name)
SKILL_ID=$(_jread identifier)
SKILL_VERSION=$(_jread version)
SKILL_NAME="${SKILL_NAME:-null}"
SKILL_ID="${SKILL_ID:-null}"
SKILL_VERSION="${SKILL_VERSION:-null}"

_dbg "metadata: name=$SKILL_NAME id=$SKILL_ID version=$SKILL_VERSION"

# ── Runtime context ───────────────────────────────────────────────────────────
AGENT_NAME="${AGENTBUDDY_AGENT_NAME:-null}"
AGENT_MODEL="${AGENTBUDDY_AGENT_MODEL:-null}"
USERNAME="${AGENTBUDDY_USERNAME:-null}"

# ── Session ID ────────────────────────────────────────────────────────────────
if [ -n "${AGENTBUDDY_SESSION_ID:-}" ] && \
   [ "${AGENTBUDDY_SESSION_ID}" != "null" ]; then
  SESSION_ID="$AGENTBUDDY_SESSION_ID"
else
  SESSION_ID=$(_uuid)
fi

SPAN_ID=$(_hex 8)
START_MS=$(_now_ms)

_dbg "span_id=$SPAN_ID session_id=$SESSION_ID start_ms=$START_MS"

# ── Persist state for span_invoke_end.sh ───────────────────────────────────────
SAFE_SKILL=$(printf '%s' "$SKILL_NAME" | tr -cs 'a-zA-Z0-9_-' '_')
mkdir -p "$STATE_DIR" 2>/dev/null
find "$STATE_DIR" -name '*.env' -mtime +0 -exec rm -f {} + 2>/dev/null

STATE_FILE="$STATE_DIR/${PPID}_${SAFE_SKILL}.env"
STATE_TMP="$STATE_FILE.tmp"
printf 'SPAN_ID=%s\nSTART_MS=%s\nSKILL_NAME=%s\nSKILL_ID=%s\nSKILL_VERSION=%s\nSESSION_ID=%s\nUSERNAME=%s\nAGENT_NAME=%s\nAGENT_MODEL=%s\n' \
  "$SPAN_ID" "$START_MS" "$SKILL_NAME" "$SKILL_ID" "$SKILL_VERSION" \
  "$SESSION_ID" "$USERNAME" "$AGENT_NAME" "$AGENT_MODEL" \
  > "$STATE_TMP"
mv -f "$STATE_TMP" "$STATE_FILE"

_dbg "state written to $STATE_FILE"

# ── Build and send start event ────────────────────────────────────────────────
PARAMS=$(printf '{"span_id":%s,"name":"skill.invoke","kind":"CLIENT","start_time_ms":%s,"end_time_ms":null,"attributes__skill__name":%s,"attributes__skill__id":%s,"attributes__skill__version":%s,"attributes__skill__result_status":null,"attributes__skill__result_message":null,"attributes__agent__name":%s,"attributes__agent__model":%s}' \
  "$(_jv "$SPAN_ID")" "$START_MS" \
  "$(_jv "$SKILL_NAME")" "$(_jv "$SKILL_ID")" "$(_jv "$SKILL_VERSION")" \
  "$(_jv "$AGENT_NAME")" "$(_jv "$AGENT_MODEL")")

PARAMS_ESC=$(printf '%s' "$PARAMS" | sed 's/\\/\\\\/g; s/"/\\"/g')

PAYLOAD=$(printf '[{"events":[{"event":"ai_extension_custom_event","params":"%s","local_time_ms":%s,"is_bav":1,"session_id":%s}],"user":{"user_unique_id":%s},"header":{"app_id":1009601},"verbose":1}]' \
  "$PARAMS_ESC" "$START_MS" "$(_jv "$SESSION_ID")" "$(_jv_str "$USERNAME")")

_send_event "$PAYLOAD"
_dbg "span_invoke_start.sh done"
