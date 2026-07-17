#!/bin/sh
# resolve_username.sh — Heuristic local username resolution
# Sourced (not executed directly). Sets AGENTBUDDY_USERNAME if not already set.
# Never crashes. Every strategy returns "" on failure. OS fallback guarantees a value.
# Requires: sh, grep, sed, id. Optional: klist, git, timeout/gtimeout.

# ── Guard: ensure _dbg is available ──────────────────────────────────────────
command -v _dbg >/dev/null 2>&1 || _dbg() { :; }

# ── Short-circuit if already resolved ─────────────────────────────────────────
if [ -n "${AGENTBUDDY_USERNAME:-}" ]; then
  _dbg "resolve_username: already set (AGENTBUDDY_USERNAME=$AGENTBUDDY_USERNAME)"
  return 0 2>/dev/null || exit 0
fi

# ── Portable timeout wrapper ──────────────────────────────────────────────────
_run_with_timeout() {
  _secs="$1"; shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$_secs" "$@" 2>/dev/null
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$_secs" "$@" 2>/dev/null
  else
    # POSIX fallback: background + kill after delay
    "$@" 2>/dev/null &
    _pid=$!
    ( sleep "$_secs" && kill "$_pid" 2>/dev/null ) &
    _guard=$!
    wait "$_pid" 2>/dev/null
    _rc=$?
    kill "$_guard" 2>/dev/null
    wait "$_guard" 2>/dev/null
    return $_rc
  fi
}

# ── Strategy: MIRA_CURRENT_USERNAME env var ────────────────────────────────────
_strategy_mira_env() {
  _val=$(printf '%s' "${MIRA_CURRENT_USERNAME:-}" | tr -d ' \t\r\n')
  [ -n "$_val" ] && printf '%s' "$_val" && return 0
  return 1
}

# ── Strategy: AIPaaS user config (~/.aipaas/user.yml) ─────────────────────────
_strategy_aipaas() {
  _file="$HOME/.aipaas/user.yml"
  [ -f "$_file" ] || return 1
  _val=$(grep -m1 '^username:' "$_file" 2>/dev/null | sed 's/^username:[[:space:]]*//' | sed "s/^[\"']//; s/[\"']$//" | tr -d ' \t\r')
  [ -n "$_val" ] && printf '%s' "$_val" && return 0
  return 1
}

# ── Strategy: Kerberos (klist) ────────────────────────────────────────────────
_strategy_kerberos() {
  _out=$(_run_with_timeout 3 /usr/bin/klist 2>/dev/null) || return 1
  _principal=$(printf '%s' "$_out" | grep -i '[Pp]rincipal:' | head -1 | sed 's/.*[Pp]rincipal:[[:space:]]*//' | cut -d'@' -f1 | tr -d ' \t\r')
  [ -n "$_principal" ] && printf '%s' "$_principal" && return 0
  return 1
}

# ── Strategy: Git config (user.email) ─────────────────────────────────────────
_strategy_git_email() {
  _out=$(_run_with_timeout 3 git config user.email 2>/dev/null) || return 1
  _email=$(printf '%s' "$_out" | tr -d ' \t\r\n' | grep -oE '[A-Za-z0-9._+-]+@[A-Za-z0-9._+-]+' | head -1)
  [ -n "$_email" ] || return 1
  _user=$(printf '%s' "$_email" | cut -d'@' -f1)
  [ -n "$_user" ] && printf '%s' "$_user" && return 0
  return 1
}

# ── Strategy: OS fallback ─────────────────────────────────────────────────────
_strategy_os_fallback() {
  _user=$(id -un 2>/dev/null) || _user=""
  [ -n "$_user" ] && printf '%s' "$_user" && return 0
  printf 'unknown'
}

# ── Execute strategies in priority order ──────────────────────────────────────
_resolved=""

_dbg "resolve_username: trying aipaas strategy"
_resolved=$(_strategy_aipaas) 2>/dev/null

if [ -z "$_resolved" ]; then
  _dbg "resolve_username: trying mira env strategy"
  _resolved=$(_strategy_mira_env) 2>/dev/null
fi

if [ -z "$_resolved" ]; then
  _dbg "resolve_username: trying kerberos strategy"
  _resolved=$(_strategy_kerberos) 2>/dev/null
fi

if [ -z "$_resolved" ]; then
  _dbg "resolve_username: trying git email strategy"
  _resolved=$(_strategy_git_email) 2>/dev/null
fi

if [ -z "$_resolved" ]; then
  _dbg "resolve_username: using OS fallback"
  _resolved=$(_strategy_os_fallback) 2>/dev/null
fi

# ── Export result ─────────────────────────────────────────────────────────────
if [ -n "$_resolved" ]; then
  AGENTBUDDY_USERNAME="$_resolved"
  export AGENTBUDDY_USERNAME
  _dbg "resolve_username: resolved to '$AGENTBUDDY_USERNAME'"
fi
