"""Open a verdict file in $EDITOR for manual rubric entry."""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
from datetime import datetime, timezone
from pathlib import Path

from . import harness, mechanical, paths, validate


def _default_verdict(scenario: dict, trace: dict, mech: dict) -> dict:
    """Template pre-filled with mechanical results; judgmental items blank."""
    results = []
    for item in scenario.get("rubric", []):
        rid = item["id"]
        m = mech.get(rid)
        if m is not None:
            results.append({
                "id": rid,
                "criterion": item["criterion"],
                "type": item["type"],
                "pass": m["pass"],
                "note": m["note"],
            })
        else:
            results.append({
                "id": rid,
                "criterion": item["criterion"],
                "type": item["type"],
                "pass": None,
                "note": "",
            })
    return {
        "run_id": trace["run_id"],
        "scenario_id": trace["scenario_id"],
        "judge": os.environ.get("USER", "manual"),
        "judged_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "rubric_results": results,
        "overall": "pending",
        "notes_for_next_iteration": "",
    }


def judge(run_spec: str) -> Path:
    run_path = paths.resolve_run(run_spec)
    trace = harness.load_trace(run_path)
    scenario_path = paths.scenario_path(trace["scenario_id"])
    if not scenario_path.exists():
        raise FileNotFoundError(f"scenario file missing: {scenario_path}")
    with open(scenario_path) as f:
        scenario = json.load(f)

    mech = mechanical.evaluate(scenario, trace)
    template = _default_verdict(scenario, trace, mech)

    # Also annotate each rubric_result's `criterion` comment-style at the
    # top of the file so the human has context without bouncing files.
    preamble = [
        f"// Verdict for run {trace['run_id']} of scenario {trace['scenario_id']}.",
        f"// Final output (first 500 chars):",
    ]
    for line in (trace.get("final_output") or "")[:500].splitlines():
        preamble.append(f"//   {line}")
    preamble.append("//")
    preamble.append("// Mechanical items are pre-filled from ground_truth.")
    preamble.append("// Set `pass` on judgmental items (true/false) and set `overall`")
    preamble.append("// to pass/fail/mixed. Remove these // lines before saving if")
    preamble.append("// your editor preserves them (they get stripped on load).")

    body = "\n".join(preamble) + "\n" + json.dumps(template, indent=2) + "\n"

    with tempfile.NamedTemporaryFile(
        "w", suffix=".verdict.jsonc", delete=False,
    ) as tf:
        tf.write(body)
        tmp_path = Path(tf.name)

    editor = os.environ.get("EDITOR") or "vim"
    subprocess.call([editor, str(tmp_path)])

    text = tmp_path.read_text()
    tmp_path.unlink(missing_ok=True)

    json_lines = [ln for ln in text.splitlines() if not ln.strip().startswith("//")]
    parsed = json.loads("\n".join(json_lines))
    validate.validate_verdict(parsed)

    out_path = paths.verdict_path_for_run(run_path)
    with open(out_path, "w") as f:
        json.dump(parsed, f, indent=2)
    return out_path
