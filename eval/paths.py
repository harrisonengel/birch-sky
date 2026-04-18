"""Filesystem layout helpers for the eval harness.

The eval harness is filesystem-as-database. Everything here is built on
stable path conventions so that `ls`, `grep`, and `jq` remain the tools
of last resort for any query the CLI doesn't cover.
"""

from __future__ import annotations

from pathlib import Path

# Resolve from this file's location: eval/ is at repo root.
EVAL_DIR = Path(__file__).resolve().parent
REPO_ROOT = EVAL_DIR.parent

SCENARIOS_DIR = EVAL_DIR / "scenarios"
FIXTURES_DIR = EVAL_DIR / "fixtures"
RUNS_DIR = EVAL_DIR / "runs"
PROMPTS_DIR = RUNS_DIR / "prompts"
VERDICTS_DIR = EVAL_DIR / "verdicts"
SCHEMAS_DIR = EVAL_DIR / "schemas"


def scenario_path(scenario_id: str) -> Path:
    return SCENARIOS_DIR / f"{scenario_id}.json"


def fixture_path(fixture_id: str) -> Path:
    return FIXTURES_DIR / f"{fixture_id}.json"


def run_filename(scenario_id: str, timestamp: str, run_id: str) -> str:
    return f"{scenario_id}__{timestamp}__{run_id}.json"


def verdict_filename(scenario_id: str, timestamp: str, run_id: str) -> str:
    return f"{scenario_id}__{timestamp}__{run_id}.verdict.json"


def parse_run_filename(name: str) -> tuple[str, str, str] | None:
    """Split a run file stem back into (scenario_id, timestamp, run_id).

    Returns None if the filename doesn't match the expected shape.
    """
    stem = name.rsplit("/", 1)[-1]
    if stem.endswith(".json"):
        stem = stem[:-5]
    parts = stem.split("__")
    if len(parts) != 3:
        return None
    return parts[0], parts[1], parts[2]


def all_run_files() -> list[Path]:
    """All run files, sorted lexicographically (ISO-8601 timestamps sort)."""
    return sorted(RUNS_DIR.glob("*.json"))


def runs_for_scenario(scenario_id: str) -> list[Path]:
    return sorted(RUNS_DIR.glob(f"{scenario_id}__*.json"))


def latest_run() -> Path | None:
    files = all_run_files()
    return files[-1] if files else None


def find_run_by_id(run_id: str) -> Path | None:
    """Match any run file whose `run_id` suffix equals the given id."""
    matches = list(RUNS_DIR.glob(f"*__{run_id}.json"))
    if len(matches) == 1:
        return matches[0]
    if len(matches) > 1:
        # Multiple scenarios with the same run_id suffix — shouldn't happen
        # but be explicit so the user can sort it out.
        raise ValueError(f"ambiguous run_id {run_id}: {matches}")
    return None


def resolve_run(spec: str) -> Path:
    """'latest' or a run_id → concrete run file path."""
    if spec == "latest":
        p = latest_run()
        if p is None:
            raise FileNotFoundError("no runs yet in eval/runs/")
        return p
    p = find_run_by_id(spec)
    if p is None:
        raise FileNotFoundError(f"no run found with id {spec}")
    return p


def verdict_path_for_run(run_path: Path) -> Path:
    """Verdict file that pairs with a given run file."""
    stem = run_path.stem  # "<scenario>__<timestamp>__<run_id>"
    return VERDICTS_DIR / f"{stem}.verdict.json"
