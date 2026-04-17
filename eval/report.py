"""Tabular summary across verdict files.

Walks eval/verdicts/, aggregates pass counts per rubric id per scenario,
and prints a plain-text table. `--since` filters on judged_at.
"""

from __future__ import annotations

import json
from collections import defaultdict

from . import paths


def _load_verdicts(since_iso: str | None) -> list[dict]:
    results = []
    for p in sorted(paths.VERDICTS_DIR.glob("*.verdict.json")):
        with open(p) as f:
            v = json.load(f)
        if since_iso and v.get("judged_at", "") < since_iso:
            continue
        results.append(v)
    return results


def report(since: str | None) -> str:
    verdicts = _load_verdicts(since)
    if not verdicts:
        return "no verdicts yet"

    # scenario -> rubric_id -> [pass_bool, ...]; also track last_run date
    tallies: dict[str, dict[str, list[bool]]] = defaultdict(lambda: defaultdict(list))
    last_run: dict[str, str] = {}
    rubric_ids_by_scenario: dict[str, list[str]] = defaultdict(list)

    for v in verdicts:
        sid = v["scenario_id"]
        judged = v.get("judged_at", "")
        if judged > last_run.get(sid, ""):
            last_run[sid] = judged
        for r in v.get("rubric_results", []):
            rid = r["id"]
            if rid not in rubric_ids_by_scenario[sid]:
                rubric_ids_by_scenario[sid].append(rid)
            if r.get("pass") is not None:
                tallies[sid][rid].append(bool(r["pass"]))

    # Determine column set = union of rubric ids across scenarios, ordered
    # by first appearance in the scenario sweep.
    column_ids: list[str] = []
    for sid in sorted(tallies.keys()):
        for rid in rubric_ids_by_scenario[sid]:
            if rid not in column_ids:
                column_ids.append(rid)

    header = ["scenario"] + column_ids + ["last_run"]
    rows = [header]
    for sid in sorted(tallies.keys()):
        row = [sid]
        for rid in column_ids:
            votes = tallies[sid].get(rid, [])
            if not votes:
                row.append("-")
            else:
                row.append(f"{sum(votes)}/{len(votes)}")
        row.append(last_run.get(sid, "")[:10])  # date only
        rows.append(row)

    # Pad columns.
    widths = [max(len(r[i]) for r in rows) for i in range(len(header))]
    out_lines = []
    for r in rows:
        out_lines.append("  ".join(c.ljust(widths[i]) for i, c in enumerate(r)))
    return "\n".join(out_lines)
