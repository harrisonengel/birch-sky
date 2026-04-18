"""Pretty-print a trace file.

The buyer-facing answer (buy_listings) is the headline — that's the
sanitized, server-hydrated result that exits the wall via /api/enter.
Agent reasoning and tool calls live below as supporting evidence.
"""

from __future__ import annotations

import json
from pathlib import Path

from . import harness


def _render_buy_listings(buy_listings: list[dict]) -> str:
    if not buy_listings:
        return "(no recommendations — agent declined or failed to submit)"
    lines = []
    for i, bl in enumerate(buy_listings, 1):
        price = bl.get("price", 0)
        lines.append(f"{i}. {bl.get('seller', '(unknown seller)')} — ${price / 100:.2f}")
        lines.append(f"   id: {bl.get('id', '')}")
        desc = bl.get("listing_description", "")
        if desc:
            lines.append(f"   {desc}")
    return "\n".join(lines)


def show(run_path: Path, show_steps: bool = False, show_final: bool = False) -> str:
    trace = harness.load_trace(run_path)

    buy_listings = trace.get("buy_listings") or []

    # --final: only print what the buyer sees, nothing else.
    if show_final and not show_steps:
        return _render_buy_listings(buy_listings)

    lines: list[str] = []
    lines.append("=== buyer-facing answer (what /api/enter returns) ===")
    lines.append(_render_buy_listings(buy_listings))
    if trace.get("hydration_error"):
        lines.append("")
        lines.append(f"!! hydration_error: {trace['hydration_error']}")
    lines.append("")

    lines.append("=== run metadata ===")
    lines.append(f"run_id:              {trace.get('run_id')}")
    lines.append(f"scenario_id:         {trace.get('scenario_id')}")
    lines.append(f"timestamp:           {trace.get('timestamp')}")
    lines.append(f"code_version:        {trace.get('code_version')}")
    cfg = trace.get("harness_config", {})
    lines.append(f"model:               {cfg.get('model')}")
    lines.append(f"market_platform_url: {cfg.get('market_platform_url')}")
    lines.append(f"prompt_hash:         {cfg.get('system_prompt_hash')}")
    lines.append(f"fixture_hash:        {cfg.get('seed_fixture_hash')}")
    lines.append(f"note:                {cfg.get('note') or '(none)'}")
    lines.append(f"steps:               {len(trace.get('agent_steps') or [])}")
    recs = trace.get("recommendations") or []
    lines.append(f"recommendations:     {len(recs)}")
    if trace.get("exception"):
        exc = trace["exception"]
        lines.append(f"exception:           {exc['type']}: {exc['message']}")

    if recs:
        lines.append("")
        lines.append("=== raw recommendations (ids the agent submitted, pre-hydration) ===")
        for r in recs:
            lines.append(f"  - seller_id={r.get('seller_id')}  listing_id={r.get('listing_id')}")

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
                if step.get("output") is not None:
                    lines.append(f"output:")
                    lines.append(str(step.get("output", "")))

    return "\n".join(lines)
