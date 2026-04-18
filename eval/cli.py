"""argparse dispatcher for the ie_eval CLI.

Subcommands: run, run-all, show, diff, judge, report, new-scenario.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import fnmatch
import json
import sys
from pathlib import Path

from . import diff as diff_mod
from . import harness as harness_mod
from . import judge as judge_mod
from . import mechanical, paths, report as report_mod, scaffold, show as show_mod


def _scenario_ids(filter_glob: str | None = None) -> list[str]:
    out = []
    for f in sorted(paths.SCENARIOS_DIR.glob("*.json")):
        sid = f.stem
        if filter_glob and not fnmatch.fnmatch(sid, filter_glob):
            continue
        out.append(sid)
    return out


def cmd_run(args) -> int:
    if not args.note:
        print("warning: --note is empty; reports will be hard to read later",
              file=sys.stderr)
    try:
        path, trace = harness_mod.run_scenario(
            args.scenario_id,
            note=args.note or "",
            config_path=Path(args.config) if args.config else None,
        )
    except FileNotFoundError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    recs = trace.get("recommendations") or []
    bls = trace.get("buy_listings") or []
    print(f"-> {path.relative_to(paths.REPO_ROOT)}")
    summary = f"{len(recs)} recommendation(s), {len(bls)} hydrated"
    if trace.get("hydration_error"):
        summary += f"  [hydration_error: {trace['hydration_error']}]"
    print(f"   run_id={trace['run_id']}  {summary}")
    if trace.get("exception"):
        print(f"   exception: {trace['exception']['type']}: "
              f"{trace['exception']['message']}", file=sys.stderr)
        return 1
    return 0


def _run_one_for_runall(scenario_id: str, note: str, config_path: Path | None) -> tuple[str, Path | None, dict | None, str | None]:
    try:
        path, trace = harness_mod.run_scenario(scenario_id, note, config_path)
        return scenario_id, path, trace, None
    except Exception as e:  # noqa: BLE001
        return scenario_id, None, None, f"{type(e).__name__}: {e}"


def cmd_run_all(args) -> int:
    ids = _scenario_ids(args.filter)
    if not ids:
        print("no scenarios matched", file=sys.stderr)
        return 2

    config_path = Path(args.config) if args.config else None
    note = args.note or ""
    results: list[tuple[str, Path | None, dict | None, str | None]] = []

    if args.parallel and args.parallel > 1:
        with concurrent.futures.ProcessPoolExecutor(max_workers=args.parallel) as ex:
            futures = {
                ex.submit(_run_one_for_runall, sid, note, config_path): sid
                for sid in ids
            }
            for fut in concurrent.futures.as_completed(futures):
                results.append(fut.result())
        results.sort(key=lambda r: r[0])
    else:
        for sid in ids:
            results.append(_run_one_for_runall(sid, note, config_path))

    any_mech_fail = False
    for sid, path, trace, err in results:
        if err:
            print(f"{sid:<32} [ERROR] {err}")
            any_mech_fail = True
            continue
        scenario = json.loads(paths.scenario_path(sid).read_text())
        mech = mechanical.evaluate(scenario, trace)
        grid = mechanical.render_grid(scenario, mech)
        for v in mech.values():
            if v is not None and not v["pass"]:
                any_mech_fail = True
        note_col = f"run_id={trace['run_id']}"
        if trace.get("exception"):
            note_col += f"  ← exception:{trace['exception']['type']}"
        print(f"{sid:<32} {grid}  {note_col}")

    return 1 if any_mech_fail else 0


def cmd_show(args) -> int:
    path = paths.resolve_run(args.run_spec)
    print(show_mod.show(path, show_steps=args.steps, show_final=args.final))
    return 0


def cmd_diff(args) -> int:
    try:
        print(diff_mod.diff(args.scenario_id, args.run_a, args.run_b))
    except FileNotFoundError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    return 0


def cmd_judge(args) -> int:
    try:
        out = judge_mod.judge(args.run_spec)
    except FileNotFoundError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    print(f"-> {out.relative_to(paths.REPO_ROOT)}")
    return 0


def cmd_report(args) -> int:
    print(report_mod.report(args.since))
    return 0


def cmd_new_scenario(args) -> int:
    try:
        out = scaffold.new_scenario(args.scenario_id)
    except FileExistsError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    print(f"-> {out.relative_to(paths.REPO_ROOT)}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="ie_eval")
    sub = p.add_subparsers(dest="subcommand", required=True)

    r = sub.add_parser("run", help="run a scenario against the eval env")
    r.add_argument("scenario_id")
    r.add_argument("--config", help="path to eval config yaml")
    r.add_argument("--note", help="short description of what you changed")
    r.set_defaults(func=cmd_run)

    ra = sub.add_parser("run-all", help="run every scenario (or glob filter)")
    ra.add_argument("--filter", help="glob matched against scenario_id")
    ra.add_argument("--parallel", type=int, default=0, help="process pool size")
    ra.add_argument("--config", help="path to eval config yaml")
    ra.add_argument("--note", help="short description of what you changed")
    ra.set_defaults(func=cmd_run_all)

    sh = sub.add_parser("show", help="pretty-print a trace")
    sh.add_argument("run_spec", help="run_id or 'latest'")
    sh.add_argument("--steps", action="store_true")
    sh.add_argument("--final", action="store_true")
    sh.set_defaults(func=cmd_show)

    d = sub.add_parser("diff", help="diff two runs of the same scenario")
    d.add_argument("scenario_id")
    d.add_argument("run_a", nargs="?")
    d.add_argument("run_b", nargs="?")
    d.set_defaults(func=cmd_diff)

    j = sub.add_parser("judge", help="open a verdict file in $EDITOR")
    j.add_argument("run_spec", help="run_id or 'latest'")
    j.set_defaults(func=cmd_judge)

    rp = sub.add_parser("report", help="summary table across verdicts")
    rp.add_argument("--since", help="ISO date lower bound on judged_at")
    rp.set_defaults(func=cmd_report)

    ns = sub.add_parser("new-scenario", help="scaffold a new scenario file")
    ns.add_argument("scenario_id")
    ns.set_defaults(func=cmd_new_scenario)

    return p


def main(argv: list[str]) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return args.func(args)
