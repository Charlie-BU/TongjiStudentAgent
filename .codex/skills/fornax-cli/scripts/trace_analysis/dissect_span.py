#!/usr/bin/env python3
"""
Span Dissector — deep structural breakdown of a single span for AI analysis.

Usage:
    python3 dissect_span.py <trace_json_file> --span-id <SPAN_ID> [--max-content-len 5000] [--sections identity,anomalies]

Produces a structured, AI-friendly report with selectable sections:
  - Span identity & position in trace
  - Timeline context (what happened before/after)
  - Input/Output structural breakdown (messages parsed, tool_calls extracted)
  - Upstream model analysis (for tool spans)
  - Diagnostic signals & anomaly flags
"""
import json
import sys
import argparse

from trace_common import (
    safe_int, safe_float, load_trace, build_tree, format_duration,
    truncate, try_parse_json, resolve_span_id, find_upstream_model,
)


# === Section 名称映射 ===

SECTION_MAP = {
    "identity": ["1_identity"],
    "performance": ["2_performance"],
    "input": ["3_input_breakdown"],
    "output": ["4_output_breakdown"],
    "timeline": ["5_timeline_context"],
    "upstream": ["6_upstream_model"],
    "children": ["7_children"],
    "anomalies": ["8_anomalies"],
    "tags": ["9_other_tags", "9_environment"],
}


# === Input/Output 解析 ===

def _extract_text_from_content(content):
    """从 content 中提取纯文本，兼容 string / list[{type,text}] 两种格式"""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for c in content:
            if isinstance(c, str):
                parts.append(c)
            elif isinstance(c, dict):
                text = c.get("text", "")
                if text:
                    parts.append(text)
        return "\n".join(parts)
    return str(content)


def _dissect_openai_messages(messages, max_len):
    """解析 OpenAI 格式: {"messages": [...]}"""
    result = {
        "format": "openai",
        "total_messages": len(messages),
        "message_summary": [],
    }

    for i, msg in enumerate(messages):
        role = msg.get("role", "?")
        content = msg.get("content", "")
        tool_calls = msg.get("tool_calls", [])
        tool_call_id = msg.get("tool_call_id", "")

        entry = {"index": i, "role": role}

        if role == "system":
            entry["content_preview"] = truncate(str(content), max_len)
            entry["content_length"] = len(str(content))
        elif role == "assistant" and tool_calls:
            entry["has_tool_calls"] = True
            entry["tool_calls"] = []
            for tc in tool_calls:
                fn = tc.get("function", {})
                tc_info = {
                    "id": tc.get("id", ""),
                    "name": fn.get("name", "?"),
                    "arguments_preview": truncate(fn.get("arguments", ""), 500),
                }
                entry["tool_calls"].append(tc_info)
            if content:
                entry["content_preview"] = truncate(str(content), 500)
        elif role == "tool":
            entry["tool_call_id"] = tool_call_id
            entry["content_preview"] = truncate(str(content), max_len)
            entry["content_length"] = len(str(content))
        elif role == "user":
            entry["content_preview"] = truncate(str(content), max_len)
            entry["content_length"] = len(str(content))
        else:
            entry["content_preview"] = truncate(str(content), 800)
            entry["content_length"] = len(str(content))

        result["message_summary"].append(entry)

    roles = [m.get("role", "?") for m in messages]
    result["role_sequence"] = " → ".join(roles)
    result["system_prompt_count"] = sum(1 for r in roles if r == "system")
    result["user_message_count"] = sum(1 for r in roles if r == "user")
    result["assistant_message_count"] = sum(1 for r in roles if r == "assistant")
    result["tool_result_count"] = sum(1 for r in roles if r == "tool")

    return result


def _dissect_anthropic_input(items, max_len):
    """解析 Anthropic/Responses API 格式: {"input": [...]}"""
    result = {
        "format": "anthropic_responses",
        "total_items": len(items),
        "message_summary": [],
    }

    type_counts = {}
    for item in items:
        t = item.get("type", item.get("role", "unknown"))
        type_counts[t] = type_counts.get(t, 0) + 1
    result["type_distribution"] = type_counts

    for i, item in enumerate(items):
        item_type = item.get("type")
        role = item.get("role")

        if role and not item_type:
            content = item.get("content", "")
            text = _extract_text_from_content(content)
            entry = {"index": i, "role": role, "content_length": len(text)}
            if role in ("developer", "system"):
                entry["content_preview"] = truncate(text, max_len)
            elif role == "user":
                entry["content_preview"] = truncate(text, max_len)
            elif role == "assistant":
                entry["content_preview"] = truncate(text, 800)
            else:
                entry["content_preview"] = truncate(text, 800)
            result["message_summary"].append(entry)

        elif item_type == "reasoning":
            encrypted = item.get("encrypted_content", "")
            summary = item.get("summary", [])
            entry = {"index": i, "type": "reasoning"}
            if encrypted:
                entry["encrypted"] = True
                entry["encrypted_length"] = len(encrypted)
            if summary:
                entry["summary"] = truncate(str(summary), 500)
            result["message_summary"].append(entry)

        elif item_type == "function_call":
            entry = {
                "index": i,
                "type": "function_call",
                "name": item.get("name", "?"),
                "call_id": item.get("call_id", ""),
            }
            args = item.get("arguments", "")
            try:
                args_parsed = json.loads(args) if isinstance(args, str) else args
                entry["arguments_preview"] = truncate(json.dumps(args_parsed, ensure_ascii=False), 500)
            except Exception:
                entry["arguments_preview"] = truncate(str(args), 500)
            result["message_summary"].append(entry)

        elif item_type == "function_call_output":
            output = item.get("output", "")
            output_text = _extract_text_from_content(output)
            entry = {
                "index": i,
                "type": "function_call_output",
                "call_id": item.get("call_id", ""),
                "output_length": len(output_text),
                "output_preview": truncate(output_text, max_len),
            }
            result["message_summary"].append(entry)

        else:
            entry = {"index": i, "type": item_type or "unknown"}
            if role:
                entry["role"] = role
            content = item.get("content", "")
            text = _extract_text_from_content(content)
            if text:
                entry["content_preview"] = truncate(text, 800)
                entry["content_length"] = len(text)
            result["message_summary"].append(entry)

    seq = []
    for item in items:
        t = item.get("type")
        r = item.get("role")
        if r and not t:
            seq.append(r)
        elif t == "reasoning":
            seq.append("reasoning")
        elif t == "function_call":
            name = item.get("name", "?")
            seq.append(f"call:{name}")
        elif t == "function_call_output":
            seq.append("call_output")
        else:
            seq.append(t or "?")
    result["sequence"] = " → ".join(seq)

    return result


def dissect_model_input(input_data, max_len):
    parsed = try_parse_json(input_data)
    if not isinstance(parsed, dict):
        return {"raw": truncate(str(parsed), max_len)}

    messages = parsed.get("messages")
    if isinstance(messages, list) and messages:
        return _dissect_openai_messages(messages, max_len)

    input_items = parsed.get("input")
    if isinstance(input_items, list) and input_items:
        return _dissect_anthropic_input(input_items, max_len)

    return {"raw": truncate(json.dumps(parsed, ensure_ascii=False), max_len)}


def _dissect_openai_output(choices, max_len):
    """解析 OpenAI 格式: {"choices": [...]}"""
    result = {"format": "openai", "choices": []}
    for c in choices:
        choice = {
            "index": c.get("index", 0),
            "finish_reason": c.get("finish_reason", "?"),
        }
        msg = c.get("message", {})
        if msg:
            choice["role"] = msg.get("role", "?")
            content = msg.get("content", "")
            if content:
                choice["content_preview"] = truncate(str(content), max_len)
                choice["content_length"] = len(str(content))
            tool_calls = msg.get("tool_calls", [])
            if tool_calls:
                choice["tool_calls"] = []
                for tc in tool_calls:
                    fn = tc.get("function", {})
                    choice["tool_calls"].append({
                        "id": tc.get("id", ""),
                        "name": fn.get("name", "?"),
                        "arguments_preview": truncate(fn.get("arguments", ""), 800),
                    })
        result["choices"].append(choice)
    return result


def _dissect_anthropic_output(output_items, parsed, max_len):
    """解析 Anthropic/Responses API 格式: {"output": [...]}"""
    result = {
        "format": "anthropic_responses",
        "status": parsed.get("status", "?"),
        "total_items": len(output_items),
        "output_summary": [],
    }

    usage = parsed.get("usage")
    if usage:
        result["usage"] = usage

    for i, item in enumerate(output_items):
        item_type = item.get("type", "unknown")

        if item_type == "reasoning":
            entry = {"index": i, "type": "reasoning"}
            encrypted = item.get("encrypted_content", "")
            if encrypted:
                entry["encrypted"] = True
                entry["encrypted_length"] = len(encrypted)
            summary = item.get("summary", [])
            if summary:
                entry["summary"] = truncate(str(summary), 500)
            result["output_summary"].append(entry)

        elif item_type == "message":
            content = item.get("content", [])
            text = _extract_text_from_content(content)
            entry = {
                "index": i,
                "type": "message",
                "role": item.get("role", "?"),
                "status": item.get("status", "?"),
                "content_preview": truncate(text, max_len),
                "content_length": len(text),
            }
            result["output_summary"].append(entry)

        elif item_type == "function_call":
            entry = {
                "index": i,
                "type": "function_call",
                "name": item.get("name", "?"),
                "call_id": item.get("call_id", ""),
                "status": item.get("status", "?"),
            }
            args = item.get("arguments", "")
            try:
                args_parsed = json.loads(args) if isinstance(args, str) else args
                entry["arguments_preview"] = truncate(json.dumps(args_parsed, ensure_ascii=False), 800)
            except Exception:
                entry["arguments_preview"] = truncate(str(args), 800)
            result["output_summary"].append(entry)

        else:
            entry = {"index": i, "type": item_type}
            result["output_summary"].append(entry)

    return result


def dissect_model_output(output_data, max_len):
    parsed = try_parse_json(output_data)
    if not isinstance(parsed, dict):
        return {"raw": truncate(str(parsed), max_len)}

    choices = parsed.get("choices")
    if isinstance(choices, list) and choices:
        return _dissect_openai_output(choices, max_len)

    output_items = parsed.get("output")
    if isinstance(output_items, list) and output_items:
        return _dissect_anthropic_output(output_items, parsed, max_len)

    return {"raw": truncate(json.dumps(parsed, ensure_ascii=False), max_len)}


def dissect_tool_input(input_data, max_len):
    parsed = try_parse_json(input_data)
    if isinstance(parsed, dict):
        result = {}
        for k, v in parsed.items():
            result[k] = truncate(str(v), max_len)
        return result
    return {"raw": truncate(str(parsed), max_len)}


def dissect_tool_output(output_data, max_len):
    parsed = try_parse_json(output_data)
    if isinstance(parsed, str):
        return {"content_preview": truncate(parsed, max_len), "content_length": len(parsed)}
    if isinstance(parsed, dict):
        result = {}
        for k, v in parsed.items():
            result[k] = truncate(json.dumps(v, ensure_ascii=False) if isinstance(v, (dict, list)) else str(v), max_len)
        return result
    return {"raw": truncate(str(parsed), max_len)}


# === 上下文与诊断 ===

def get_timeline_context(spans, target_span, by_id, children):
    parent_id = target_span.get("parent_id", "")
    if not parent_id:
        search_parent = ""
        for s in spans:
            if s.get("parent_id", "") not in set(x["span_id"] for x in spans):
                search_parent = s.get("parent_id", "")
                break
        siblings = sorted(spans, key=lambda s: safe_int(s.get("started_at", 0)))
    else:
        search_id = parent_id
        parent = by_id.get(parent_id)
        if parent and parent.get("span_type") not in ("graph", "Chain", "Agent"):
            search_id = parent.get("parent_id", parent_id)
        siblings = sorted(children.get(search_id, []), key=lambda s: safe_int(s.get("started_at", 0)))

    target_idx = -1
    for i, s in enumerate(siblings):
        if s["span_id"] == target_span["span_id"]:
            target_idx = i
            break

    if target_idx == -1:
        siblings = sorted(children.get(parent_id, []), key=lambda s: safe_int(s.get("started_at", 0)))
        for i, s in enumerate(siblings):
            if s["span_id"] == target_span["span_id"]:
                target_idx = i
                break

    before = []
    after = []
    if target_idx >= 0:
        for s in siblings[:target_idx]:
            ct = s.get("custom_tags", {}) or {}
            before.append({
                "span_id": s["span_id"],
                "name": s.get("span_name", "?"),
                "type": s.get("span_type", "?"),
                "duration": format_duration(s.get("duration", 0)),
                "status": s.get("status", "?"),
                "model": ct.get("model_name", ""),
            })
        for s in siblings[target_idx + 1:]:
            ct = s.get("custom_tags", {}) or {}
            after.append({
                "span_id": s["span_id"],
                "name": s.get("span_name", "?"),
                "type": s.get("span_type", "?"),
                "duration": format_duration(s.get("duration", 0)),
                "status": s.get("status", "?"),
                "model": ct.get("model_name", ""),
            })

    return {
        "position": f"{target_idx + 1}/{len(siblings)}" if target_idx >= 0 else "unknown",
        "before": before[-5:],
        "after": after[:5],
    }


def detect_anomalies(span, spans, by_id, children):
    flags = []
    ct = span.get("custom_tags", {}) or {}
    stype = span.get("span_type", "")
    dur = safe_int(span.get("duration", 0))
    status = span.get("status", "")

    if status != "success":
        if dur < 500 and stype == "model":
            flags.append("🔴 Model error with very short duration (<500ms) — likely API rejection/rate limit, not timeout")
        elif dur > 10000 and stype == "model":
            flags.append("🔴 Model error with long duration (>10s) — likely timeout")
        else:
            flags.append(f"🔴 Span failed with status={status}")

    if stype == "model":
        itok = safe_int(ct.get("input_tokens", 0))
        otok = safe_int(ct.get("output_tokens", 0))
        if itok > 30000:
            flags.append(f"⚠️ Very high input tokens ({itok}) — possible context window pressure")
        if otok == 0 and status == "success":
            flags.append("⚠️ Zero output tokens despite success — possible empty response")
        latency_first = safe_int(ct.get("latency_first_resp", 0))
        if latency_first > 0 and dur > 0:
            gen_time = dur - latency_first
            if gen_time > latency_first * 3:
                flags.append(f"⚠️ Generation time ({gen_time}ms) much longer than first-token latency ({latency_first}ms) — large output")

    if stype == "tool":
        output = str(span.get("output", ""))
        if not output or output in ("", "null", "None"):
            flags.append("⚠️ Tool has no output — possible crash or missing report")

    parent = by_id.get(span.get("parent_id", ""))
    if parent:
        parent_dur = safe_int(parent.get("duration", 0))
        if parent_dur > 0 and dur > parent_dur * 0.8:
            flags.append(f"⚠️ This span consumes {dur/parent_dur*100:.0f}% of parent's duration — dominant bottleneck")

    if stype == "model" and status != "success":
        parent_for_siblings = by_id.get(span.get("parent_id", ""))
        if parent_for_siblings and parent_for_siblings.get("span_type") not in ("graph", "Chain", "Agent"):
            search_span_parent = parent_for_siblings.get("parent_id", "")
        else:
            search_span_parent = span.get("parent_id", "")

        sibs = sorted(children.get(search_span_parent, []), key=lambda s: safe_int(s.get("started_at", 0)))
        error_streak = 0
        for s in sibs:
            if s.get("span_type") == "model" and s.get("status") != "success":
                error_streak += 1
        if error_streak >= 2:
            flags.append(f"🔴 {error_streak} consecutive model errors in this scope — likely rate limiting or API issue")

    return flags


# === 报告生成 ===

def build_report(span, spans, by_id, children, max_len, sections=None):
    """构建结构化的 span 分析报告。sections 为 None 时输出全部，否则只保留指定 section。"""
    ct = span.get("custom_tags", {}) or {}
    st = span.get("system_tags", {}) or {}
    stype = span.get("span_type", "")

    report = {}

    # === Section 1: Identity ===
    report["1_identity"] = {
        "span_id": span["span_id"],
        "span_name": span.get("span_name", "?"),
        "span_type": stype,
        "status": span.get("status", "?"),
        "status_code": span.get("status_code", "?"),
        "duration_ms": safe_int(span.get("duration", 0)),
        "duration_human": format_duration(span.get("duration", 0)),
        "started_at": span.get("started_at", "?"),
    }
    if ct.get("model_name"):
        report["1_identity"]["model_name"] = ct["model_name"]
    if ct.get("model_provider"):
        report["1_identity"]["model_provider"] = ct["model_provider"]
    if ct.get("agent_name"):
        report["1_identity"]["agent_name"] = ct["agent_name"]

    # === Section 2: Token & Performance (model only) ===
    if stype == "model":
        perf = {}
        for key in ("input_tokens", "output_tokens", "tokens", "reasoning_tokens", "input_cached_tokens", "latency_first_resp"):
            val = ct.get(key)
            if val is not None:
                perf[key] = safe_int(val)
        if ct.get("stream"):
            perf["streaming"] = ct["stream"]
        if ct.get("call_options"):
            try:
                perf["call_options"] = json.loads(ct["call_options"])
            except Exception:
                perf["call_options"] = ct["call_options"]
        report["2_performance"] = perf

    # === Section 3: Input Breakdown ===
    input_data = span.get("input", "")
    if stype == "model":
        report["3_input_breakdown"] = dissect_model_input(input_data, max_len)
    elif stype == "tool":
        report["3_input_breakdown"] = dissect_tool_input(input_data, max_len)
    else:
        parsed = try_parse_json(input_data)
        report["3_input_breakdown"] = {"raw": truncate(json.dumps(parsed, ensure_ascii=False) if isinstance(parsed, (dict, list)) else str(parsed), max_len)}

    # === Section 4: Output Breakdown ===
    output_data = span.get("output", "")
    if stype == "model":
        report["4_output_breakdown"] = dissect_model_output(output_data, max_len)
    elif stype == "tool":
        report["4_output_breakdown"] = dissect_tool_output(output_data, max_len)
    else:
        parsed = try_parse_json(output_data)
        report["4_output_breakdown"] = {"raw": truncate(json.dumps(parsed, ensure_ascii=False) if isinstance(parsed, (dict, list)) else str(parsed), max_len)}

    # === Section 5: Timeline Context ===
    report["5_timeline_context"] = get_timeline_context(spans, span, by_id, children)

    # === Section 6: Upstream Model (for tool/ToolsNode) ===
    if stype in ("tool", "ToolsNode"):
        upstream = find_upstream_model(span, by_id, children)
        if upstream:
            uct = upstream.get("custom_tags", {}) or {}
            report["6_upstream_model"] = {
                "span_id": upstream["span_id"],
                "name": upstream.get("span_name", "?"),
                "model_name": uct.get("model_name", "?"),
                "duration_ms": safe_int(upstream.get("duration", 0)),
                "status": upstream.get("status", "?"),
                "input_tokens": safe_int(uct.get("input_tokens", 0)),
                "output_tokens": safe_int(uct.get("output_tokens", 0)),
                "hint": "This model's output triggered the tool call. Check its output for the tool_call parameters.",
            }
        else:
            report["6_upstream_model"] = {"message": "No upstream model found"}

    # === Section 7: Children ===
    child_spans = sorted(children.get(span["span_id"], []), key=lambda s: safe_int(s.get("started_at", 0)))
    if child_spans:
        report["7_children"] = []
        for c in child_spans:
            cct = c.get("custom_tags", {}) or {}
            report["7_children"].append({
                "span_id": c["span_id"],
                "name": c.get("span_name", "?"),
                "type": c.get("span_type", "?"),
                "duration_ms": safe_int(c.get("duration", 0)),
                "status": c.get("status", "?"),
                "model_name": cct.get("model_name", ""),
            })

    # === Section 8: Anomaly Detection ===
    anomalies = detect_anomalies(span, spans, by_id, children)
    if anomalies:
        report["8_anomalies"] = anomalies

    # === Section 9: Key Tags ===
    interesting_tags = {}
    skip_keys = {"duration", "eino_ext_versions", "eino_run_info_component", "eino_run_info_name",
                 "eino_run_info_type", "fornax_space_id", "psm_env", "request_env",
                 "input_tokens", "output_tokens", "tokens", "reasoning_tokens",
                 "input_cached_tokens", "latency_first_resp", "model_name", "model_provider",
                 "agent_name", "stream", "call_options"}
    for k, v in ct.items():
        if k not in skip_keys:
            interesting_tags[k] = truncate(str(v), 200)
    if interesting_tags:
        report["9_other_tags"] = interesting_tags

    env_info = {}
    for k in ("env", "region", "cluster", "pod_name"):
        if st.get(k):
            env_info[k] = st[k]
    if env_info:
        report["9_environment"] = env_info

    if sections is not None:
        report = {k: v for k, v in report.items() if k in sections}

    return report


def main():
    parser = argparse.ArgumentParser(description="Deep dissection of a single span from a Fornax trace")
    parser.add_argument("trace_file", help="Path to trace JSON file")
    parser.add_argument("--span-id", required=True, help="Target span ID (supports prefix match)")
    parser.add_argument("--max-content-len", type=int, default=5000, help="Max chars for content fields (default: 5000)")
    parser.add_argument("--sections", help="Comma-separated sections to include (default: all). Available: " + ",".join(sorted(SECTION_MAP.keys())))
    args = parser.parse_args()

    spans = load_trace(args.trace_file)
    by_id, children, _ = build_tree(spans)
    max_len = args.max_content_len

    # 解析 --sections
    section_keys = None
    if args.sections:
        section_keys = set()
        for name in args.sections.split(","):
            name = name.strip()
            if name not in SECTION_MAP:
                print(json.dumps({"error": f"Unknown section '{name}'", "available": sorted(SECTION_MAP.keys())}, indent=2))
                sys.exit(1)
            for key in SECTION_MAP[name]:
                section_keys.add(key)

    # 前缀匹配
    try:
        resolved_id = resolve_span_id(by_id, args.span_id)
    except ValueError as e:
        print(json.dumps({"error": str(e)}, indent=2))
        sys.exit(1)

    if not resolved_id:
        print(json.dumps({"error": f"span_id '{args.span_id}' not found", "available_count": len(by_id)}, indent=2))
        sys.exit(1)

    span = by_id[resolved_id]
    report = build_report(span, spans, by_id, children, max_len, sections=section_keys)
    print(json.dumps(report, indent=2, ensure_ascii=False, default=str))


if __name__ == "__main__":
    main()
