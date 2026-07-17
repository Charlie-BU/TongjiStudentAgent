#!/bin/sh
# span_invoke_end.sh — Send a complete skill.invoke end event to MCS.
# Usage: ./span_invoke_end.sh [result_status] [result_message]
#   result_status: success | error | abort | timeout | skipped  (default: success)
#   result_message: free-form string, or omit
# Reads skill metadata from state file, then meta.json, then falls back to nulls.
# Requires: sh, curl, od, tr, grep, cut, sed, date. No python, no jq, no openssl.

set +e

SCRIPT_DIR="$(dirname "$0")"
# shellcheck source=telemetry_lib.sh
. "$SCRIPT_DIR/telemetry_lib.sh"
# shellcheck source=resolve_username.sh
. "$SCRIPT_DIR/resolve_username.sh"

_dbg "span_invoke_end.sh begin"

RESULT_STATUS="${1:-success}"
RESULT_MESSAGE="${2:-}"

case "$RESULT_STATUS" in
  success|error|abort|timeout|skipped) ;;
  *) _dbg "invalid result_status='$RESULT_STATUS', defaulting to 'error'"
     RESULT_STATUS="error" ;;
esac

END_MS=$(_now_ms)

# ── Degraded-mode defaults ────────────────────────────────────────────────────
SPAN_ID=$(_hex 8)
START_MS="$END_MS"
SKILL_NAME="null"; SKILL_ID="null"; SKILL_VERSION="null"
SESSION_ID="${AGENTBUDDY_SESSION_ID:-null}"
USERNAME="${AGENTBUDDY_USERNAME:-null}"
AGENT_NAME="${AGENTBUDDY_AGENT_NAME:-null}"
AGENT_MODEL="${AGENTBUDDY_AGENT_MODEL:-null}"

# ── Tier 1: restore from state file ──────────────────────────────────────────
# shellcheck disable=SC2034
META="$SCRIPT_DIR/meta.json"

_sn=$(_jread name); SKILL_NAME="${_sn:-null}"
SAFE_SKILL=$(printf '%s' "$SKILL_NAME" | tr -cs 'a-zA-Z0-9_-' '_')
STATE_FILE="${STATE_DIR}/${PPID}_${SAFE_SKILL}.env"

_dbg "looking for state file: $STATE_FILE"

if [ -f "$STATE_FILE" ]; then
  _dbg "state file found, restoring"
  _v() { grep "^${1}=" "$STATE_FILE" | head -1 | cut -d= -f2-; }
  _r=$(_v SPAN_ID);       [ -n "$_r" ] && SPAN_ID="$_r"
  _r=$(_v START_MS);      [ -n "$_r" ] && START_MS="$_r"
  _r=$(_v SKILL_NAME);    [ -n "$_r" ] && SKILL_NAME="$_r"
  _r=$(_v SKILL_ID);      [ -n "$_r" ] && SKILL_ID="$_r"
  _r=$(_v SKILL_VERSION); [ -n "$_r" ] && SKILL_VERSION="$_r"
  _r=$(_v SESSION_ID);    [ -n "$_r" ] && SESSION_ID="$_r"
  _r=$(_v USERNAME);      [ -n "$_r" ] && USERNAME="$_r"
  _r=$(_v AGENT_NAME);    [ -n "$_r" ] && AGENT_NAME="$_r"
  _r=$(_v AGENT_MODEL);   [ -n "$_r" ] && AGENT_MODEL="$_r"
  rm -f "$STATE_FILE" 2>/dev/null
else
  _dbg "state file not found, using degraded defaults"
fi

# ── Tier 2: fill remaining nulls from meta.json ──────────────────────
[ "$SKILL_ID"      = "null" ] && { _r=$(_jread identifier); [ -n "$_r" ] && SKILL_ID="$_r"; }
[ "$SKILL_VERSION" = "null" ] && { _r=$(_jread version);    [ -n "$_r" ] && SKILL_VERSION="$_r"; }

# ── Tier 3: generate session_id if still missing ─────────────────────────────
if [ -z "$SESSION_ID" ] || [ "$SESSION_ID" = "null" ]; then
  if [ -n "${AGENTBUDDY_SESSION_ID:-}" ] && \
     [ "${AGENTBUDDY_SESSION_ID}" != "null" ]; then
    SESSION_ID="$AGENTBUDDY_SESSION_ID"
  else
    SESSION_ID=$(_uuid)
  fi
fi

# ── Stale state file cleanup ─────────────────────────────────────────────────
find "$STATE_DIR" -name '*.env' -mtime +0 -exec rm -f {} + 2>/dev/null

_dbg "span_id=$SPAN_ID session_id=$SESSION_ID start_ms=$START_MS end_ms=$END_MS"
_dbg "result_status=$RESULT_STATUS result_message=$RESULT_MESSAGE"

# ── Build and send end event ──────────────────────────────────────────────────
PARAMS=$(printf '{"span_id":%s,"name":"skill.invoke","kind":"CLIENT","start_time_ms":%s,"end_time_ms":%s,"attributes__skill__name":%s,"attributes__skill__id":%s,"attributes__skill__version":%s,"attributes__skill__result_status":%s,"attributes__skill__result_message":%s,"attributes__agent__name":%s,"attributes__agent__model":%s}' \
  "$(_jv "$SPAN_ID")" "$START_MS" "$END_MS" \
  "$(_jv "$SKILL_NAME")" "$(_jv "$SKILL_ID")" "$(_jv "$SKILL_VERSION")" \
  "$(_jv "$RESULT_STATUS")" "$(_jv "$RESULT_MESSAGE")" \
  "$(_jv "$AGENT_NAME")" "$(_jv "$AGENT_MODEL")")

PARAMS_ESC=$(printf '%s' "$PARAMS" | sed 's/\\/\\\\/g; s/"/\\"/g')

PAYLOAD=$(printf '[{"events":[{"event":"ai_extension_custom_event","params":"%s","local_time_ms":%s,"is_bav":1,"session_id":%s}],"user":{"user_unique_id":%s},"header":{"app_id":1009601},"verbose":1}]' \
  "$PARAMS_ESC" "$END_MS" "$(_jv "$SESSION_ID")" "$(_jv_str "$USERNAME")")

_send_event "$PAYLOAD"
_dbg "span_invoke_end.sh done"
