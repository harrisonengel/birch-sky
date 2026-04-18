"""Diff two runs of the same scenario.

Default to the last two runs for the scenario. Produce a config delta
header, then unified diffs of the buyer-facing output (recommendations +
buy_listings) and serialized agent_steps. Use `delta` if present, else
shell out to `diff -u`.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from . import paths


def _config_delta(a: dict, b: dict) -> list[str]:
    lines = []
    keys = sorted(set(a.keys()) | set(b.keys()))
    for k in keys:
        av, bv = a.get(k), b.get(k)
        if av != bv:
            lines.append(f"  {k}: {av!r} -> {bv!r}")
    return lines


def _unified_diff(a_text: str, b_text: str, a_label: str, b_label: str) -> str:
    """Run `delta` if available, else `diff -u`. Return captured output."""
    tool = "delta" if shutil.which("delta") else "diff"
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fa, \
         tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fb:
        fa.write(a_text)
        fb.write(b_text)
        fa_path, fb_path = fa.name, fb.name
    try:
        if tool == "delta":
            out = subprocess.run(
                ["delta", "--paging=never", "--file-style=omit",
                 "--hunk-header-style=omit", fa_path, fb_path],
                capture_output=True, text=True,
            )
            return out.stdout
        else:
            out = subprocess.run(
                ["diff", "-u", f"--label={a_label}", f"--label={b_label}",
                 fa_path, fb_path],
                capture_output=True, text=True,
            )
            return out.stdout
    finally:
        Path(fa_path).unlink(missing_ok=True)
        Path(fb_path).unlink(missing_ok=True)


def diff(scenario_id: str, run_a: str | None, run_b: str | None) -> str:
    """Compare two runs. Defaults to the last two runs of the scenario."""
    if run_a and run_b:
        a_path = paths.resolve_run(run_a)
        b_path = paths.resolve_run(run_b)
    else:
        scenario_runs = paths.runs_for_scenario(scenario_id)
        if len(scenario_runs) < 2:
            raise FileNotFoundError(
                f"need at least 2 runs of {scenario_id}; have {len(scenario_runs)}"
            )
        a_path, b_path = scenario_runs[-2], scenario_runs[-1]

    from . import harness  # lazy: harness pulls yaml; not needed for _config_delta
    a = harness.load_trace(a_path)
    b = harness.load_trace(b_path)

    lines = []
    lines.append(f"A: {a_path.name}")
    lines.append(f"B: {b_path.name}")
    lines.append("")
    lines.append("=== Config delta ===")
    cd = _config_delta(a.get("harness_config", {}), b.get("harness_config", {}))
    lines.extend(cd if cd else ["  (no changes)"])

    lines.append("")
    lines.append("=== Recommendations diff ===")
    a_recs = json.dumps(a.get("recommendations") or [], indent=2, sort_keys=True)
    b_recs = json.dumps(b.get("recommendations") or [], indent=2, sort_keys=True)
    lines.append(_unified_diff(
        a_recs, b_recs,
        f"A/{a_path.stem}/recommendations", f"B/{b_path.stem}/recommendations",
    ).rstrip() or "(identical)")

    lines.append("")
    lines.append("=== Buyer-facing diff (buy_listings) ===")
    a_bl = json.dumps(a.get("buy_listings") or [], indent=2, sort_keys=True)
    b_bl = json.dumps(b.get("buy_listings") or [], indent=2, sort_keys=True)
    lines.append(_unified_diff(
        a_bl, b_bl,
        f"A/{a_path.stem}/buy_listings", f"B/{b_path.stem}/buy_listings",
    ).rstrip() or "(identical)")

    a_steps = json.dumps(a.get("agent_steps") or [], indent=2)
    b_steps = json.dumps(b.get("agent_steps") or [], indent=2)
    lines.append("")
    lines.append(f"=== Step count: {len(a.get('agent_steps') or [])} "
                 f"-> {len(b.get('agent_steps') or [])} ===")
    step_diff = _unified_diff(
        a_steps, b_steps,
        f"A/{a_path.stem}/agent_steps", f"B/{b_path.stem}/agent_steps",
    )
    if step_diff.strip():
        lines.append(step_diff.rstrip())
    return "\n".join(lines)
