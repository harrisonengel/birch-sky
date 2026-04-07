import os
import subprocess

from .models import Task, AgentResult


def review_gate(task: Task, result: AgentResult) -> bool:
    """CLI review gate. Returns True if approved, False if rejected."""
    print(f"\n{'=' * 60}")
    print(f"REVIEW: {task.goal}")
    print(f"Type: {task.type}  ID: {task.id}")
    print(f"{'=' * 60}")

    # Show output preview
    lines = result.output.splitlines()
    preview = "\n".join(lines[:50])
    print(preview)
    if len(lines) > 50:
        print(f"\n... ({len(lines) - 50} more lines)")

    if result.proposed_tasks:
        print(f"\nProposed follow-ups: {len(result.proposed_tasks)}")
        for pt in result.proposed_tasks:
            print(f"  - [{pt.get('type', '?')}] {pt.get('goal', '?')}")

    if result.open_questions:
        print(f"\nOpen questions:")
        for oq in result.open_questions:
            print(f"  ? {oq}")

    print(f"{'=' * 60}")

    while True:
        resp = input("[a]pprove / [r]eject / [v]iew full: ").strip().lower()
        if resp in ("a", "approve"):
            return True
        if resp in ("r", "reject"):
            return False
        if resp in ("v", "view"):
            out_path = f"outputs/{task.id}/output.md"
            if os.path.exists(out_path):
                editor = os.environ.get("EDITOR", "less")
                subprocess.run([editor, out_path])
            else:
                print(result.output)
