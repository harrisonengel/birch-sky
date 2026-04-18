"""Scenario loader + agent invocation wrapper.

Responsible for the full `ie_eval run` lifecycle, minus the CLI wrapping:
- Load scenario + fixture
- Build a Session from buyer_context
- Call the existing harness.runner.execute with a trace list
- Hydrate the agent's structured recommendations into buy_listings — this
  is the sanitized view the buyer sees outside the wall. Free-form LLM text
  does not cross the wall; only ids do.
- Write trace file to eval/runs/
- Write the resolved system prompt to eval/runs/prompts/{hash}.txt once
"""

from __future__ import annotations

import hashlib
import json
import secrets
import subprocess
import sys
import traceback
from datetime import datetime, timezone
from pathlib import Path

# Make src/ importable so we can reach harness.* without a package install.
_REPO = Path(__file__).resolve().parent.parent
_SRC = _REPO / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from harness.config import HarnessConfig  # noqa: E402
from harness.runner import execute as agent_execute  # noqa: E402
from harness.session import from_context  # noqa: E402
from harness import tools as agent_tools  # noqa: E402

from . import paths, validate


def _iso_timestamp() -> tuple[str, str]:
    """Return (filesystem_ts, iso_ts). FS form has dashes; ISO has colons."""
    now = datetime.now(timezone.utc).replace(microsecond=0)
    iso = now.strftime("%Y-%m-%dT%H:%M:%SZ")
    fs = now.strftime("%Y-%m-%dT%H-%M-%SZ")
    return fs, iso


def _code_version() -> str:
    try:
        sha = subprocess.check_output(
            ["git", "rev-parse", "--short", "HEAD"],
            cwd=_REPO, stderr=subprocess.DEVNULL,
        ).decode().strip()
        dirty = subprocess.call(
            ["git", "diff", "--quiet"], cwd=_REPO,
        ) != 0
        return f"{sha}{'-dirty' if dirty else ''}"
    except (subprocess.CalledProcessError, FileNotFoundError):
        return "unknown"


def _sha256(text: str) -> str:
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()


def _build_session_context(buyer: dict, scenario_id: str, fixture_id: str) -> dict:
    """Map the spec's buyer_context onto the agent's starting_context keys."""
    parts = []
    if "budget_usd" in buyer and buyer["budget_usd"] is not None:
        parts.append(f"budget: ${buyer['budget_usd']}")
    for c in buyer.get("constraints", []) or []:
        parts.append(str(c))
    return {
        "background": f"Evaluation scenario {scenario_id} "
                      f"(fixture: {fixture_id}).",
        "goal": buyer.get("task", ""),
        "constraints": "; ".join(parts) if parts else "",
    }


def _load_scenario(scenario_id: str) -> dict:
    path = paths.scenario_path(scenario_id)
    if not path.exists():
        raise FileNotFoundError(f"scenario not found: {path}")
    with open(path) as f:
        obj = json.load(f)
    validate.validate_scenario(obj)
    return obj


def _load_fixture(fixture_id: str) -> tuple[dict, str]:
    """Return (fixture_obj, sha256_hex). Missing fixture is not fatal — the
    agent runs against whatever's currently seeded in the eval env."""
    path = paths.fixture_path(fixture_id)
    if not path.exists():
        return {}, ""
    raw = path.read_text()
    return json.loads(raw), _sha256(raw)


def load_eval_config(config_path: Path | None) -> HarnessConfig:
    """Reuse the existing harness.config loader on the eval config file."""
    from harness import config as config_module
    if config_path is None:
        config_path = paths.EVAL_DIR / "config.eval.yaml"
    if not config_path.exists():
        example = paths.EVAL_DIR / "config.eval.example.yaml"
        raise FileNotFoundError(
            f"{config_path} not found. Copy {example} and fill in values."
        )
    return config_module.load(str(config_path))


def run_scenario(
    scenario_id: str,
    note: str,
    config_path: Path | None,
) -> tuple[Path, dict]:
    """Run one scenario end-to-end and write the trace. Returns (trace_path, trace)."""
    scenario = _load_scenario(scenario_id)
    fixture, fixture_hash = _load_fixture(scenario["market_fixture"])

    config = load_eval_config(config_path)
    session_context = _build_session_context(
        scenario["buyer_context"], scenario_id, scenario["market_fixture"],
    )
    session = from_context(session_context)

    prompt_hash = _sha256(session.instructions)
    prompt_file = paths.PROMPTS_DIR / f"{prompt_hash.split(':',1)[1]}.txt"
    if not prompt_file.exists():
        prompt_file.write_text(session.instructions)

    run_id = secrets.token_hex(3)  # 6 hex chars ≈ 16M space, plenty per scenario
    fs_ts, iso_ts = _iso_timestamp()

    trace_steps: list[dict] = []
    user_input = scenario["buyer_context"]["task"]

    recommendations: list[dict] = []
    buy_listings: list[dict] = []
    hydration_error: str | None = None
    exc_payload: dict | None = None
    try:
        recommendations = agent_execute(config, session, user_input, trace=trace_steps)
    except Exception as e:  # noqa: BLE001 — spec says failures are data
        exc_payload = {
            "type": type(e).__name__,
            "message": str(e),
            "traceback": traceback.format_exc(),
        }

    # Hydrate whatever recommendations the agent submitted into the user-facing
    # buy_listings shape. This is exactly what /api/enter returns to the buyer.
    # Hydration failure is tracked separately from an agent-loop exception so
    # we can tell the two apart (wall upheld / wall rejected a bogus id).
    if recommendations and exc_payload is None:
        agent_tools.configure(market_platform_url=config.market_platform_url)
        try:
            buy_listings = agent_tools.hydrate_buy_listings(recommendations)
        except agent_tools.HydrationError as he:
            hydration_error = str(he)
        except Exception as he:  # noqa: BLE001 — network / server 5xx etc.
            hydration_error = f"{type(he).__name__}: {he}"

    trace = {
        "run_id": run_id,
        "scenario_id": scenario_id,
        "timestamp": iso_ts,
        "code_version": _code_version(),
        "harness_config": {
            "model": config.model,
            "system_prompt_hash": prompt_hash,
            "market_platform_url": config.market_platform_url,
            "seed_fixture_hash": fixture_hash,
            "note": note,
        },
        "resolved_inputs": {
            "buyer_context": scenario["buyer_context"],
            "market_fixture": scenario["market_fixture"],
        },
        "agent_steps": trace_steps,
        "recommendations": recommendations,
        "buy_listings": buy_listings,
        "hydration_error": hydration_error,
        "exception": exc_payload,
    }
    if fixture:
        # Record which sellers/listings were available at run time without
        # bloating the trace with full descriptions.
        trace["resolved_inputs"]["fixture_snapshot"] = {
            "sellers": [s.get("external_id") for s in fixture.get("sellers", [])],
            "listings": [l.get("external_id") for l in fixture.get("listings", [])],
        }

    out_path = paths.RUNS_DIR / paths.run_filename(scenario_id, fs_ts, run_id)
    with open(out_path, "w") as f:
        json.dump(trace, f, indent=2)
    return out_path, trace


def load_trace(run_path: Path) -> dict:
    with open(run_path) as f:
        return json.load(f)
