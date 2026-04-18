"""Scaffold a new scenario file from a template and open in $EDITOR."""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

from . import paths


def _template(scenario_id: str) -> dict:
    return {
        "scenario_id": scenario_id,
        "description": "",
        "buyer_context": {
            "task": "",
            "budget_usd": 0,
            "constraints": [],
        },
        "market_fixture": "market_small_v1",
        "ground_truth": {
            "correct_seller_ids": [],
            "correct_dataset_ids": [],
            "must_not_retain_fields": [],
            "expected_conclusion_contains": [],
        },
        "rubric": [
            {"id": "r1", "criterion": "Agent identifies the correct dataset", "type": "mechanical"},
            {"id": "r2", "criterion": "Final output contains required facts", "type": "mechanical"},
            {"id": "r3", "criterion": "Reasoning is sound and complete", "type": "judgmental"},
        ],
        "notes": "",
    }


def new_scenario(scenario_id: str) -> Path:
    out = paths.scenario_path(scenario_id)
    if out.exists():
        raise FileExistsError(f"{out} already exists")
    out.write_text(json.dumps(_template(scenario_id), indent=2) + "\n")
    editor = os.environ.get("EDITOR") or "vim"
    subprocess.call([editor, str(out)])
    return out
