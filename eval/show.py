"""Pretty-print a trace file."""

from __future__ import annotations

import json
from pathlib import Path

from . import harness


def show(run_path: Path, show_steps: bool = False, show_final: bool = False) -> str:
    trace = harness.load_trace(run_path)

    if show_final and not show_steps:
        return trace.get("final_output", "") or ""

    lines: list[str] = []
    lines.append(f"run_id:         {trace.get('run_id')}")
    lines.append(f"scenario_id:    {trace.get('scenario_id')}")
    lines.append(f"timestamp:      {trace.get('timestamp')}")
    lines.append(f"code_version:   {trace.get('code_version')}")
    cfg = trace.get("harness_config", {})
    lines.append(f"model:          {cfg.get('model')}")
    lines.append(f"opensearch:     {cfg.get('opensearch_url')} ({cfg.get('opensearch_index')})")
    lines.append(f"prompt_hash:    {cfg.get('system_prompt_hash')}")
    lines.append(f"fixture_hash:   {cfg.get('seed_fixture_hash')}")
    lines.append(f"note:           {cfg.get('note') or '(none)'}")
    lines.append(f"steps:          {len(trace.get('agent_steps') or [])}")
    if trace.get("exception"):
        lines.append(f"exception:      {trace['exception']['type']}: {trace['exception']['message']}")

    if show_steps:
        lines.append("")
        lines.append("=== agent_steps ===")
        for step in trace.get("agent_steps", []) or []:
            lines.append(f"\n-- step {step['step']} [{step['kind']}] --")
            if step["kind"] == "reasoning":
                lines.append(step.get("content", ""))
            else:
                lines.append(f"tool:   {step.get('tool')}")
                lines.append(f"input:  {json.dumps(step.get('input'), indent=2)}")
                lines.append(f"output:")
                lines.append(str(step.get("output", "")))

    lines.append("")
    lines.append("=== final_output ===")
    lines.append(trace.get("final_output") or "(empty)")
    return "\n".join(lines)
