"""CLI review gate for spec/decide tasks."""

import os
import subprocess

from .models import Task


def _git_diff_stat() -> str:
    try:
        proc = subprocess.run(
            ["git", "diff", "--stat"],
            capture_output=True,
            text=True,
            check=False,
        )
        return proc.stdout.strip()
    except FileNotFoundError:
        return ""


def review_gate(task: Task, result: dict) -> bool:
    """CLI review gate. Returns True if approved, False if rejected."""
    print(f"\n{'=' * 60}")
    print(f"REVIEW: {task.goal}")
    print(f"Type: {task.type.value}  ID: {task.id}  Session: {result.get('session_id', '?')[:8]}")
    print(f"{'=' * 60}\n")

    # Show final assistant message
    message = result.get("result", "") or "(no final message)"
    lines = message.splitlines()
    preview = "\n".join(lines[:60])
    print(preview)
    if len(lines) > 60:
        print(f"\n... ({len(lines) - 60} more lines)")

    # Show git diff stat if we're in a repo
    diff_stat = _git_diff_stat()
    if diff_stat:
        print(f"\n--- Uncommitted changes ---\n{diff_stat}")

    print(f"\n{'=' * 60}")

    while True:
        resp = input("[a]pprove / [r]eject / [d]iff / [v]iew result file: ").strip().lower()
        if resp in ("a", "approve"):
            return True
        if resp in ("r", "reject"):
            return False
        if resp in ("d", "diff"):
            subprocess.run(["git", "diff"], check=False)
        elif resp in ("v", "view"):
            out_path = f"outputs/{task.id}/result.md"
            if os.path.exists(out_path):
                editor = os.environ.get("EDITOR", "less")
                subprocess.run([editor, out_path], check=False)
            else:
                print("(no result file yet)")
