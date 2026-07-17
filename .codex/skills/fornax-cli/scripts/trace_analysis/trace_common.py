#!/usr/bin/env python3
"""
Common utilities for trace analysis scripts.

Shared by analyze_trace.py and dissect_span.py.
"""
import json
from collections import defaultdict


def safe_int(v, default=0):
    try:
        return int(v)
    except (TypeError, ValueError):
        return default


def safe_float(v, default=0.0):
    try:
        return float(v)
    except (TypeError, ValueError):
        return default


def load_trace(path):
    with open(path) as f:
        return json.load(f)


def build_tree(spans):
    """构建 span 树，返回 (by_id, children, roots)"""
    by_id = {s["span_id"]: s for s in spans}
    children = defaultdict(list)
    for s in spans:
        children[s.get("parent_id", "")].append(s)
    all_ids = set(by_id.keys())
    roots = [s for s in spans if s.get("parent_id", "") not in all_ids]
    return by_id, children, roots


def format_duration(ms):
    ms = safe_int(ms)
    if ms >= 60000:
        return f"{ms/1000:.1f}s ({ms}ms)"
    if ms >= 1000:
        return f"{ms/1000:.2f}s"
    return f"{ms}ms"


def truncate(s, max_len):
    s = str(s)
    if len(s) <= max_len:
        return s
    return s[:max_len] + f"... [truncated, total {len(s)} chars]"


def try_parse_json(s):
    if not isinstance(s, str):
        return s
    s = s.strip()
    if s[:1] in ("{", "["):
        try:
            return json.loads(s)
        except Exception:
            pass
    return s


def resolve_span_id(by_id, span_id_or_prefix):
    """按完整 ID 或前缀匹配 span。

    返回匹配的完整 span_id，找不到返回 None。
    前缀有多条匹配时抛 ValueError。
    """
    if span_id_or_prefix in by_id:
        return span_id_or_prefix
    matches = [sid for sid in by_id if sid.startswith(span_id_or_prefix)]
    if len(matches) == 1:
        return matches[0]
    if len(matches) > 1:
        raise ValueError(
            f"Ambiguous span ID prefix '{span_id_or_prefix}', matches {len(matches)} spans: "
            + ", ".join(sorted(matches)[:10])
        )
    return None


def find_upstream_model(target_span, by_id, children):
    """查找触发某个 tool/ToolsNode 的上游 model span。

    在同级 span 中反向查找最近的 model 类型 span。
    """
    search_span = target_span
    if target_span.get("span_type") in ("tool",):
        parent = by_id.get(target_span.get("parent_id", ""))
        if parent and parent.get("span_type") not in ("model", "graph", "Chain", "Agent"):
            search_span = parent

    parent_id = search_span.get("parent_id", "")
    if not parent_id:
        return None

    siblings = sorted(children.get(parent_id, []), key=lambda s: safe_int(s.get("started_at", 0)))
    last_model = None
    for sib in siblings:
        if sib["span_id"] == search_span["span_id"]:
            break
        if sib.get("span_type") == "model":
            last_model = sib
    return last_model
