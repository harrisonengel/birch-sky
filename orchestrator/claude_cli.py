"""Thin wrapper around the `claude` CLI in headless (`-p`) mode.

The orchestrator trusts `.claude/settings.local.json` for tool permissions,
so we do not pass `--allowedTools`. Each task runs as one headless turn;
multi-turn work happens across loop iterations via `--resume`.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from typing import Optional

from .models import Task


def build_prompt(task: Task, dep_output_paths: list[str]) -> str:
    """Build the user prompt for a fresh task run.

    CLAUDE.md is auto-loaded by Claude Code; we only add task-specific framing.
    """
    parts = [f"# Task: {task.goal}"]

    if task.acceptance_criteria:
        parts.append(f"\n## Acceptance criteria\n{task.acceptance_criteria}")

    if task.context_refs:
        parts.append("\n## Relevant files")
        for ref in task.context_refs:
            parts.append(f"- `{ref}`")

    if dep_output_paths:
        parts.append("\n## Prior task outputs")
        for p in dep_output_paths:
            parts.append(f"- `{p}`")

    if task.notes:
        parts.append(f"\n## Notes\n{task.notes}")

    parts.append(f"\n## Task type\n`{task.type.value}`")

    parts.append(
        "\n## When done\n"
        "Write a concise final message summarizing what you did. "
        "If you have follow-up tasks to propose, list them under `## Proposed follow-ups` "
        "as `- [type] goal` lines (type ∈ spec|implement|review|test|decide). "
        "If you have open questions for a human, list them under `## Open questions`."
    )

    return "\n".join(parts)


def _run_claude(prompt: str, session_id: Optional[str] = None, cwd: Optional[str] = None) -> dict:
    """Invoke `claude -p` and return a normalized result dict.

    Returns:
        {
          "ok": bool,
          "exit_code": int,
          "session_id": str | None,
          "result": str,      # final assistant message
          "raw": dict | None, # full JSON payload from claude
          "error": str | None,
        }
    """
    cmd = ["claude", "-p", prompt, "--output-format", "json"]
    if session_id:
        cmd.extend(["--resume", session_id])

    try:
        proc = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            cwd=cwd,
            check=False,
        )
    except FileNotFoundError:
        return {
            "ok": False,
            "exit_code": 127,
            "session_id": None,
            "result": "",
            "raw": None,
            "error": "`claude` CLI not found in PATH. Install Claude Code: https://claude.ai/code",
        }

    if proc.returncode != 0:
        return {
            "ok": False,
            "exit_code": proc.returncode,
            "session_id": None,
            "result": "",
            "raw": None,
            "error": (proc.stderr or proc.stdout or "claude exited non-zero").strip(),
        }

    try:
        data = json.loads(proc.stdout)
    except json.JSONDecodeError as e:
        return {
            "ok": False,
            "exit_code": proc.returncode,
            "session_id": None,
            "result": "",
            "raw": None,
            "error": f"Failed to parse claude JSON output: {e}. stdout={proc.stdout[:500]}",
        }

    return {
        "ok": True,
        "exit_code": 0,
        "session_id": data.get("session_id"),
        "result": data.get("result", ""),
        "raw": data,
        "error": None,
    }


def run_task(task: Task, dep_output_paths: list[str]) -> dict:
    """Run a fresh task turn. Returns the claude result dict."""
    prompt = build_prompt(task, dep_output_paths)
    return _run_claude(prompt, session_id=task.session_id)


def resume_with_feedback(session_id: str, feedback: str) -> dict:
    """Continue a session with reviewer feedback."""
    prompt = (
        f"The reviewer requested changes before accepting your output:\n\n"
        f"{feedback}\n\n"
        f"Please address each point and produce a revised result. "
        f"End with a concise summary of what you changed."
    )
    return _run_claude(prompt, session_id=session_id)


def save_result(task_id: str, result: dict) -> str:
    """Persist the assistant's final message to outputs/<task-id>/result.md.

    Returns the path so the orchestrator can set task.output_path.
    """
    out_dir = f"outputs/{task_id}"
    os.makedirs(out_dir, exist_ok=True)
    out_path = f"{out_dir}/result.md"
    with open(out_path, "w") as f:
        f.write(result.get("result", ""))
    # Also dump raw JSON for debugging
    if result.get("raw"):
        with open(f"{out_dir}/raw.json", "w") as f:
            json.dump(result["raw"], f, indent=2)
    return out_path
