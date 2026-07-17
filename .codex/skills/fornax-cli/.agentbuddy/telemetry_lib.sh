#!/bin/sh
# telemetry_lib.sh — Shared helpers for span_invoke_start.sh / span_invoke_end.sh
# Sourced (not executed directly). Provides: _dbg, _now_ms, _hex, _uuid, _jread, _jv, _send_event
# Requires: sh, curl, awk, od, tr, grep, cut, sed, date. No python, no jq, no openssl.

# ── Debug helper ──────────────────────────────────────────────────────────────
_dbg() {
  [ "${AGENTBUDDY_DEBUG:-0}" = "1" ] && printf '[ai-ext-tel] %s\n' "$*" >&2
  return 0
}

# ── Portable millisecond timestamp ────────────────────────────────────────────
_now_ms() {
  _t=$(date +%s%3N 2>/dev/null)
  case "$_t" in *N*|*%*|"") _t=$(( $(date +%s) * 1000 )) ;; esac
  printf '%s' "$_t"
}

# ── Portable random hex (POSIX od) with fallback ─────────────────────────────
_hex() {
  _out=$(od -An -tx1 -N"$1" /dev/urandom 2>/dev/null | tr -d ' \n\t')
  if [ -z "$_out" ]; then
    _seed=$(( $$ + $(date +%s) ))
    _out=""
    _i=0
    while [ "$_i" -lt "$1" ]; do
      _seed=$(( (_seed * 1103515245 + 12345) % 2147483648 ))
      _out="${_out}$(printf '%02x' "$(( (_seed / 65536) % 256 ))")"
      _i=$(( _i + 1 ))
    done
  fi
  printf '%s' "$_out"
}

# ── RFC 4122 UUID v4 ─────────────────────────────────────────────────────────
_uuid() {
  _h=$(_hex 16)
  _u1=$(printf '%s' "$_h" | cut -c1-8)
  _u2=$(printf '%s' "$_h" | cut -c9-12)
  _u3="4$(printf '%s' "$_h" | cut -c14-16)"
  _vc=$(printf '%s' "$_h" | cut -c17)
  case "$_vc" in
    0|4|8|c|C) _vc=8 ;; 1|5|9|d|D) _vc=9 ;;
    2|6|a|A|e|E) _vc=a ;; 3|7|b|B|f|F) _vc=b ;;
  esac
  _u4="${_vc}$(printf '%s' "$_h" | cut -c18-20)"
  _u5=$(printf '%s' "$_h" | cut -c21-32)
  printf '%s-%s-%s-%s-%s' "$_u1" "$_u2" "$_u3" "$_u4" "$_u5"
}

# ── Read a string value from a flat JSON file ────────────────────────────────
# Usage: _jread <key>   (uses $META as the file path)
_jread() { grep "\"${1}\"[[:space:]]*:" "$META" 2>/dev/null | head -1 | sed 's/.*"'"$1"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/'; }

# ── JSON value formatter (proper escaping) ────────────────────────────────────
_jv() {
  if [ -z "$1" ] || [ "$1" = "null" ]; then printf 'null'
  else printf '"%s"' "$(printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g')"; fi
}

_jv_str() {
  printf '"%s"' "$(printf '%s' "${1:-null}" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g')"
}

# ── POST event to MCS ────────────────────────────────────────────────────────
# Usage: _send_event <payload>
_send_event() {
  _dbg "payload: $1"
  if [ "${AGENTBUDDY_DEBUG:-0}" = "1" ]; then
    curl -s \
      -X POST 'https://mcs.zijieapi.com/list?aid=1009601' \
      -H 'accept: */*' \
      -H 'content-type: application/json; charset=UTF-8' \
      -H 'referer: https://skills.bytedance.net/' \
      --max-time 5 --retry 2 \
      --data-raw "$1" >&2 || true
    printf '\n' >&2
  else
    curl -s -o /dev/null \
      -X POST 'https://mcs.zijieapi.com/list?aid=1009601' \
      -H 'accept: */*' \
      -H 'content-type: application/json; charset=UTF-8' \
      -H 'referer: https://skills.bytedance.net/' \
      --max-time 5 --retry 2 \
      --data-raw "$1" 2>/dev/null || true
  fi
}

# ── State directory ───────────────────────────────────────────────────────────
# shellcheck disable=SC2034
STATE_DIR="${TMPDIR:-/tmp}/ai_ext_tel"
