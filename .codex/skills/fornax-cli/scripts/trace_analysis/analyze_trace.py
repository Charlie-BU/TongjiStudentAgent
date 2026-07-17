#!/usr/bin/env python3
"""
Trace Analysis Tool — analyze a fornax-cli trace JSON file.

Usage:
    python3 analyze_trace.py <trace_json_file> [--span-id <SPAN_ID>] [--mode tree|stats|detail|full|upstream|errors] [--top N] [--top-type TYPE]

Modes:
    tree     — print the span tree (supports --span-id for subtree)
    stats    — statistical summary (token, duration, error, tool/model/agent breakdown)
    detail   — detailed info for a specific span (requires --span-id)
    upstream — find the upstream model that triggered a tool/ToolsNode span (requires --span-id)
    errors   — automatic error chain analysis (root cause identification + cascade grouping)
    full     — tree + stats combined (default)
"""
import json
import sys
import argparse
from collections import defaultdict, Counter

from trace_common import (
    safe_int, load_trace, build_tree, format_duration,
    resolve_span_id, find_upstream_model,
)


def print_tree(span, children, indent=0, lines=None):
    if lines is None:
        lines = []
    sid = span["span_id"]
    name = span.get("span_name", "?")
    stype = span.get("span_type", "?")
    dur = safe_int(span.get("duration", 0))
    status = span.get("status", "")
    ct = span.get("custom_tags", {}) or {}
    model = ct.get("model_name", "")

    err_mark = " ❌" if status != "success" else ""
    model_mark = f" [{model}]" if model else ""
    tokens = ""
    itok = safe_int(ct.get("input_tokens"))
    otok = safe_int(ct.get("output_tokens"))
    if itok or otok:
        tokens = f" tok={itok}+{otok}"

    prefix = "  " * indent + ("├─ " if indent > 0 else "")
    lines.append(f"{prefix}{name} ({stype}) {format_duration(dur)}{model_mark}{tokens}{err_mark} [{sid}]")

    for child in sorted(children.get(sid, []), key=lambda c: safe_int(c.get("started_at", 0))):
        print_tree(child, children, indent + 1, lines)
    return lines


def compute_stats(spans, subtree_root_id=None):
    if subtree_root_id:
        by_id, children, _ = build_tree(spans)
        target_ids = set()
        def collect(sid):
            target_ids.add(sid)
            for c in children.get(sid, []):
                collect(c["span_id"])
        collect(subtree_root_id)
        spans = [s for s in spans if s["span_id"] in target_ids]

    total_dur = max((safe_int(s.get("duration", 0)) for s in spans), default=0)
    models = [s for s in spans if s.get("span_type") == "model"]
    tools = [s for s in spans if s.get("span_type") == "tool"]
    errors = [s for s in spans if s.get("status") != "success"]

    # Model 聚合
    model_dur = defaultdict(lambda: {"count": 0, "total_ms": 0, "input_tokens": 0, "output_tokens": 0})
    for m in models:
        ct = m.get("custom_tags", {}) or {}
        name = ct.get("model_name") or m.get("span_name", "unknown")
        model_dur[name]["count"] += 1
        model_dur[name]["total_ms"] += safe_int(m.get("duration", 0))
        model_dur[name]["input_tokens"] += safe_int(ct.get("input_tokens", 0))
        model_dur[name]["output_tokens"] += safe_int(ct.get("output_tokens", 0))

    # Tool 聚合
    tool_stats = defaultdict(lambda: {"count": 0, "total_ms": 0, "success": 0, "fail": 0})
    for t in tools:
        name = t.get("span_name", "unknown")
        tool_stats[name]["count"] += 1
        tool_stats[name]["total_ms"] += safe_int(t.get("duration", 0))
        if t.get("status") == "success":
            tool_stats[name]["success"] += 1
        else:
            tool_stats[name]["fail"] += 1

    # Agent 聚合
    agent_spans = [s for s in spans if s.get("span_type") == "Agent"]
    agent_stats = defaultdict(lambda: {"count": 0, "total_ms": 0, "max_ms": 0, "errors": 0})
    for a in agent_spans:
        name = a.get("span_name", "unknown")
        dur = safe_int(a.get("duration", 0))
        agent_stats[name]["count"] += 1
        agent_stats[name]["total_ms"] += dur
        agent_stats[name]["max_ms"] = max(agent_stats[name]["max_ms"], dur)
        if a.get("status") != "success":
            agent_stats[name]["errors"] += 1

    total_input_tokens = sum(safe_int((s.get("custom_tags") or {}).get("input_tokens", 0)) for s in models)
    total_output_tokens = sum(safe_int((s.get("custom_tags") or {}).get("output_tokens", 0)) for s in models)

    model_time_pct = {}
    if total_dur > 0:
        for name, ms in model_dur.items():
            model_time_pct[name] = round(ms["total_ms"] / total_dur * 100, 1)

    tool_time_pct = {}
    if total_dur > 0:
        for name, ts in tool_stats.items():
            tool_time_pct[name] = round(ts["total_ms"] / total_dur * 100, 1)

    return {
        "total_spans": len(spans),
        "total_duration_ms": total_dur,
        "total_duration_human": format_duration(total_dur),
        "span_type_counts": dict(Counter(s.get("span_type", "unknown") for s in spans)),
        "model_stats": {k: {**v, "avg_ms": round(v["total_ms"] / v["count"], 1) if v["count"] else 0, "time_pct": model_time_pct.get(k, 0)} for k, v in model_dur.items()},
        "tool_stats": {k: {**v, "time_pct": tool_time_pct.get(k, 0)} for k, v in tool_stats.items()},
        "agent_stats": {k: {
            "count": v["count"],
            "total_ms": v["total_ms"],
            "avg_ms": round(v["total_ms"] / v["count"], 1) if v["count"] else 0,
            "max_ms": v["max_ms"],
            "errors": v["errors"],
        } for k, v in agent_stats.items()},
        "token_summary": {
            "total_input_tokens": total_input_tokens,
            "total_output_tokens": total_output_tokens,
            "total_tokens": total_input_tokens + total_output_tokens,
        },
        "error_spans": [
            {
                "span_id": s["span_id"],
                "name": s.get("span_name", "?"),
                "type": s.get("span_type", "?"),
                "status": s.get("status", "?"),
                "status_code": s.get("status_code", "?"),
                "duration_ms": safe_int(s.get("duration", 0)),
                "output_preview": str(s.get("output", ""))[:300],
            }
            for s in errors
        ],
        "error_count": len(errors),
    }


def get_top_spans(spans, top_n, top_type=None):
    """获取 Top N 最慢 span，可按 span_type 过滤"""
    filtered = spans
    if top_type:
        type_map = {
            "model": ["model"],
            "tool": ["tool"],
            "agent": ["Agent"],
            "chain": ["Chain"],
            "graph": ["graph"],
        }
        allowed_types = type_map.get(top_type.lower(), [top_type])
        filtered = [s for s in spans if s.get("span_type") in allowed_types]

    ranked = sorted(filtered, key=lambda s: safe_int(s.get("duration", 0)), reverse=True)
    results = []
    for s in ranked[:top_n]:
        ct = s.get("custom_tags", {}) or {}
        results.append({
            "span_id": s["span_id"],
            "name": s.get("span_name", "?"),
            "type": s.get("span_type", "?"),
            "duration_ms": safe_int(s.get("duration", 0)),
            "status": s.get("status", "?"),
            "input_tokens": safe_int(ct.get("input_tokens", 0)),
            "output_tokens": safe_int(ct.get("output_tokens", 0)),
        })
    return results


def get_span_detail(spans, span_id):
    by_id, children, _ = build_tree(spans)
    span = by_id.get(span_id)
    if not span:
        return {"error": f"span_id '{span_id}' not found"}

    ct = span.get("custom_tags", {}) or {}
    st = span.get("system_tags", {}) or {}

    child_meta = []
    for c in sorted(children.get(span_id, []), key=lambda s: safe_int(s.get("started_at", 0))):
        cct = c.get("custom_tags", {}) or {}
        child_meta.append({
            "span_id": c["span_id"],
            "name": c.get("span_name", "?"),
            "type": c.get("span_type", "?"),
            "duration_ms": safe_int(c.get("duration", 0)),
            "status": c.get("status", "?"),
            "model_name": cct.get("model_name", ""),
        })

    parent = by_id.get(span.get("parent_id", ""))
    parent_info = None
    if parent:
        pct = parent.get("custom_tags", {}) or {}
        parent_info = {
            "span_id": parent["span_id"],
            "name": parent.get("span_name", "?"),
            "type": parent.get("span_type", "?"),
            "model_name": pct.get("model_name", ""),
        }

    siblings = []
    if span.get("parent_id"):
        for sib in sorted(children.get(span["parent_id"], []), key=lambda s: safe_int(s.get("started_at", 0))):
            if sib["span_id"] != span_id:
                siblings.append({
                    "span_id": sib["span_id"],
                    "name": sib.get("span_name", "?"),
                    "type": sib.get("span_type", "?"),
                    "duration_ms": safe_int(sib.get("duration", 0)),
                })

    input_str = span.get("input", "")
    output_str = span.get("output", "")
    try:
        input_parsed = json.loads(input_str) if isinstance(input_str, str) and input_str.strip()[:1] in ("{", "[") else input_str
    except Exception:
        input_parsed = input_str
    try:
        output_parsed = json.loads(output_str) if isinstance(output_str, str) and output_str.strip()[:1] in ("{", "[") else output_str
    except Exception:
        output_parsed = output_str

    return {
        "span_id": span["span_id"],
        "name": span.get("span_name", "?"),
        "type": span.get("span_type", "?"),
        "duration_ms": safe_int(span.get("duration", 0)),
        "status": span.get("status", "?"),
        "status_code": span.get("status_code", "?"),
        "started_at": span.get("started_at", "?"),
        "model_name": ct.get("model_name", ""),
        "input_tokens": ct.get("input_tokens"),
        "output_tokens": ct.get("output_tokens"),
        "custom_tags": ct,
        "system_tags": st,
        "input": input_parsed,
        "output": output_parsed,
        "parent": parent_info,
        "children_meta": child_meta,
        "siblings": siblings,
    }


# === 输出格式化 ===

def _print_stats_text(stats, top_n=0, top_type=None, spans=None):
    """以文本格式打印统计信息"""
    print(f"Total spans: {stats['total_spans']}")
    print(f"Total duration: {stats['total_duration_human']}")
    print(f"Errors: {stats['error_count']}")
    print(f"\nSpan types: {stats['span_type_counts']}")
    print(f"\nTokens: input={stats['token_summary']['total_input_tokens']}, output={stats['token_summary']['total_output_tokens']}, total={stats['token_summary']['total_tokens']}")

    print("\n--- Model Stats ---")
    for name, ms in stats["model_stats"].items():
        print(f"  {name}: calls={ms['count']}, total={ms['total_ms']}ms, avg={ms['avg_ms']}ms, time={ms['time_pct']}%, tokens={ms['input_tokens']}+{ms['output_tokens']}")

    print("\n--- Tool Stats ---")
    for name, ts in stats["tool_stats"].items():
        sr = ts['success'] / ts['count'] * 100 if ts['count'] else 0
        print(f"  {name}: calls={ts['count']}, total={ts['total_ms']}ms, time={ts['time_pct']}%, success_rate={sr:.0f}%")

    if stats["agent_stats"]:
        print("\n--- Agent Stats ---")
        for name, a in stats["agent_stats"].items():
            print(f"  {name}: instances={a['count']}, total={format_duration(a['total_ms'])}, avg={format_duration(a['avg_ms'])}, max={format_duration(a['max_ms'])}, errors={a['errors']}")

    if stats["error_spans"]:
        print(f"\n--- Error Spans ({stats['error_count']}) ---")
        for e in stats["error_spans"]:
            print(f"  [{e['span_id']}] {e['name']} ({e['type']}) status={e['status']} dur={e['duration_ms']}ms")
            if e["output_preview"]:
                print(f"    output: {e['output_preview'][:200]}")

    if top_n > 0 and spans is not None:
        _print_top_spans(spans, top_n, top_type)


def _print_top_spans(spans, top_n, top_type=None):
    """打印 Top N 最慢 span"""
    top = get_top_spans(spans, top_n, top_type)
    type_label = f" (type={top_type})" if top_type else ""
    print(f"\n--- Top {top_n} Slowest Spans{type_label} ---")
    for i, t in enumerate(top, 1):
        tok = ""
        if t["input_tokens"] or t["output_tokens"]:
            tok = f" tok={t['input_tokens']}+{t['output_tokens']}"
        print(f"  {i:>3}. {format_duration(t['duration_ms']):>10}  {t['span_id']}  {t['name']} ({t['type']}){tok} status={t['status']}")


# === 错误链路分析 ===

def analyze_error_chains(spans):
    """分析错误链路：识别根因错误和级联错误，按因果链分组输出"""
    by_id, children, _ = build_tree(spans)
    error_ids = {s["span_id"] for s in spans if s.get("status") != "success"}

    if not error_ids:
        return {"total_errors": 0, "independent_chains": 0, "error_chains": []}

    # 根因错误：祖先链上没有其他错误 span 的错误 span
    root_cause_ids = []
    for eid in error_ids:
        is_root = True
        pid = by_id[eid].get("parent_id", "")
        while pid and pid in by_id:
            if pid in error_ids:
                is_root = False
                break
            pid = by_id[pid].get("parent_id", "")
        if is_root:
            root_cause_ids.append(eid)

    # 为每个根因构建级联错误子树
    chains = []
    for rc_id in root_cause_ids:
        chain_spans = []

        def _collect(sid, depth=0):
            if sid in error_ids:
                s = by_id[sid]
                ct = s.get("custom_tags", {}) or {}
                chain_spans.append({
                    "span_id": s["span_id"],
                    "name": s.get("span_name", "?"),
                    "type": s.get("span_type", "?"),
                    "status": s.get("status", "?"),
                    "status_code": s.get("status_code", "?"),
                    "duration_ms": safe_int(s.get("duration", 0)),
                    "depth": depth,
                    "model_name": ct.get("model_name", ""),
                    "output_preview": str(s.get("output", ""))[:300],
                })
            for child in sorted(children.get(sid, []),
                                key=lambda c: safe_int(c.get("started_at", 0))):
                _collect(child["span_id"], depth + 1)

        _collect(rc_id)

        rc = by_id[rc_id]
        rc_ct = rc.get("custom_tags", {}) or {}

        # 构建祖先路径（从根因向上到 trace root）
        ancestor_path = []
        pid = rc.get("parent_id", "")
        while pid and pid in by_id:
            p = by_id[pid]
            ancestor_path.append({
                "span_id": p["span_id"],
                "name": p.get("span_name", "?"),
                "type": p.get("span_type", "?"),
            })
            pid = p.get("parent_id", "")

        chains.append({
            "root_cause": {
                "span_id": rc["span_id"],
                "name": rc.get("span_name", "?"),
                "type": rc.get("span_type", "?"),
                "status": rc.get("status", "?"),
                "status_code": rc.get("status_code", "?"),
                "duration_ms": safe_int(rc.get("duration", 0)),
                "model_name": rc_ct.get("model_name", ""),
                "output_preview": str(rc.get("output", ""))[:300],
            },
            "ancestor_path": ancestor_path,
            "cascade_count": len(chain_spans),
            "error_spans": chain_spans,
        })

    chains.sort(key=lambda c: c["cascade_count"], reverse=True)

    return {
        "total_errors": len(error_ids),
        "independent_chains": len(chains),
        "error_chains": chains,
    }


def _print_error_chains(result):
    """以文本格式打印错误链路分析"""
    print(f"=== Error Chain Analysis ===")
    print(f"Total errors: {result['total_errors']}, Independent chains: {result['independent_chains']}")

    for i, chain in enumerate(result["error_chains"], 1):
        rc = chain["root_cause"]
        model = f" [{rc['model_name']}]" if rc["model_name"] else ""
        print(f"\n{'='*60}")
        print(f"Chain {i}: {chain['cascade_count']} error(s)")
        print(f"  ROOT CAUSE: {rc['name']} ({rc['type']}){model}")
        print(f"    span_id: {rc['span_id']}")
        print(f"    status: {rc['status']} code={rc['status_code']} dur={format_duration(rc['duration_ms'])}")
        if rc["output_preview"]:
            print(f"    output: {rc['output_preview'][:200]}")

        if chain["ancestor_path"]:
            path_str = " → ".join(f"{a['name']}({a['type']})" for a in reversed(chain["ancestor_path"]))
            print(f"  CONTEXT: {path_str} → [ROOT CAUSE]")

        if chain["cascade_count"] > 1:
            print(f"  CASCADE ({chain['cascade_count'] - 1} downstream errors):")
            for es in chain["error_spans"]:
                if es["span_id"] == rc["span_id"]:
                    continue
                indent = "    " + "  " * es["depth"]
                model_mark = f" [{es['model_name']}]" if es["model_name"] else ""
                print(f"{indent}{es['name']} ({es['type']}){model_mark} status={es['status']} dur={format_duration(es['duration_ms'])}")
                print(f"{indent}  span_id: {es['span_id']}")


def main():
    parser = argparse.ArgumentParser(description="Analyze a Fornax trace JSON file")
    parser.add_argument("trace_file", help="Path to trace JSON file")
    parser.add_argument("--span-id", help="Target span ID (supports prefix match)")
    parser.add_argument("--mode", choices=["tree", "stats", "detail", "full", "upstream", "errors"], default="full")
    parser.add_argument("--json", action="store_true", help="Output in JSON format")
    parser.add_argument("--top", type=int, default=0, help="Show top N slowest spans")
    parser.add_argument("--top-type", help="Filter --top by span type (model, tool, agent, chain, graph)")
    args = parser.parse_args()

    spans = load_trace(args.trace_file)

    # span-id 前缀匹配
    resolved_span_id = None
    if args.span_id:
        by_id, _, _ = build_tree(spans)
        try:
            resolved_span_id = resolve_span_id(by_id, args.span_id)
        except ValueError as e:
            print(f"Error: {e}", file=sys.stderr)
            sys.exit(1)
        if not resolved_span_id:
            print(f"Error: span_id '{args.span_id}' not found (tried prefix match)", file=sys.stderr)
            sys.exit(1)

    if args.mode == "tree":
        by_id, children_map, roots = build_tree(spans)
        lines = []
        if resolved_span_id:
            # 子树模式
            print_tree(by_id[resolved_span_id], children_map, 0, lines)
        else:
            for r in sorted(roots, key=lambda s: safe_int(s.get("started_at", 0))):
                print_tree(r, children_map, 0, lines)
        print("\n".join(lines))

    elif args.mode == "stats":
        stats = compute_stats(spans, resolved_span_id)
        if args.json:
            if args.top > 0:
                stats["top_spans"] = get_top_spans(spans, args.top, args.top_type)
            print(json.dumps(stats, indent=2, ensure_ascii=False))
        else:
            _print_stats_text(stats, args.top, args.top_type, spans)

    elif args.mode == "detail":
        if not resolved_span_id:
            print("Error: --span-id required for detail mode", file=sys.stderr)
            sys.exit(1)
        detail = get_span_detail(spans, resolved_span_id)
        print(json.dumps(detail, indent=2, ensure_ascii=False, default=str))

    elif args.mode == "upstream":
        if not resolved_span_id:
            print("Error: --span-id required for upstream mode", file=sys.stderr)
            sys.exit(1)
        by_id, children_map, _ = build_tree(spans)
        target = by_id[resolved_span_id]
        upstream = find_upstream_model(target, by_id, children_map)
        if upstream:
            detail = get_span_detail(spans, upstream["span_id"])
            print(json.dumps(detail, indent=2, ensure_ascii=False, default=str))
        else:
            print(json.dumps({"message": "No upstream model span found"}, indent=2))

    elif args.mode == "errors":
        result = analyze_error_chains(spans)
        if args.json:
            print(json.dumps(result, indent=2, ensure_ascii=False))
        else:
            _print_error_chains(result)

    elif args.mode == "full":
        by_id, children_map, roots = build_tree(spans)
        lines = []
        if resolved_span_id:
            print_tree(by_id[resolved_span_id], children_map, 0, lines)
        else:
            for r in sorted(roots, key=lambda s: safe_int(s.get("started_at", 0))):
                print_tree(r, children_map, 0, lines)
        print("=== Span Tree ===")
        print("\n".join(lines))
        print()

        stats = compute_stats(spans, resolved_span_id)
        print("=== Statistics ===")
        print(f"Total spans: {stats['total_spans']}, Duration: {stats['total_duration_human']}, Errors: {stats['error_count']}")
        print(f"Tokens: input={stats['token_summary']['total_input_tokens']}, output={stats['token_summary']['total_output_tokens']}, total={stats['token_summary']['total_tokens']}")
        if stats["model_stats"]:
            print("\nModel breakdown:")
            for name, ms in stats["model_stats"].items():
                print(f"  {name}: calls={ms['count']}, total={ms['total_ms']}ms, avg={ms['avg_ms']}ms, time={ms['time_pct']}%, tokens={ms['input_tokens']}+{ms['output_tokens']}")
        if stats["tool_stats"]:
            print("\nTool breakdown:")
            for name, ts in stats["tool_stats"].items():
                sr = ts['success'] / ts['count'] * 100 if ts['count'] else 0
                print(f"  {name}: calls={ts['count']}, total={ts['total_ms']}ms, time={ts['time_pct']}%, success_rate={sr:.0f}%")
        if stats["agent_stats"]:
            print("\nAgent breakdown:")
            for name, a in stats["agent_stats"].items():
                print(f"  {name}: instances={a['count']}, total={format_duration(a['total_ms'])}, avg={format_duration(a['avg_ms'])}, max={format_duration(a['max_ms'])}, errors={a['errors']}")
        if stats["error_spans"]:
            print(f"\n⚠️  {stats['error_count']} error span(s):")
            for e in stats["error_spans"]:
                print(f"  [{e['span_id']}] {e['name']} ({e['type']}) status={e['status']} dur={e['duration_ms']}ms")
        if args.top > 0:
            _print_top_spans(spans, args.top, args.top_type)


if __name__ == "__main__":
    main()
