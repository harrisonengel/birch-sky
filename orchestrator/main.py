"""Orchestrator loop.

Shells out to `claude -p` for task execution. Trusts .claude/settings.local.json
for tool permissions. Python owns: queue, review gates, Linear sync, feedback loops.
"""

import argparse
import os
import sys
import time

from . import claude_cli
from . import linear
from . import taskqueue as q
from .models import Status, Task, TaskType
from .review import review_gate

ALWAYS_GATE = {TaskType.SPEC, TaskType.DECIDE}
MAX_FEEDBACK_ITERATIONS = 5
SKILLS_DIR = ".claude/skills"


def _validate_identity(identity: str) -> None:
    """Fail fast if `.claude/skills/<identity>/SKILL.md` is missing."""
    skill_path = os.path.join(SKILLS_DIR, identity, "SKILL.md")
    if not os.path.isfile(skill_path):
        print(
            f"Unknown identity '{identity}': {skill_path} not found.",
            file=sys.stderr,
        )
        available = []
        if os.path.isdir(SKILLS_DIR):
            for name in sorted(os.listdir(SKILLS_DIR)):
                if os.path.isfile(os.path.join(SKILLS_DIR, name, "SKILL.md")):
                    available.append(name)
        if available:
            print(f"Available identities: {', '.join(available)}", file=sys.stderr)
        sys.exit(2)


def _dep_output_paths(task: Task) -> list[str]:
    all_tasks = {t.id: t for t in q.load()}
    return [
        all_tasks[d].output_path
        for d in task.dependencies
        if d in all_tasks and all_tasks[d].output_path
    ]


def triage_proposals(result_text: str, parent_task: Task, identity: str):
    """Parse '## Proposed follow-ups' section from claude's final message and triage each."""
    proposals = _parse_proposals(result_text)
    if not proposals:
        return

    print(f"\nFound {len(proposals)} proposed follow-up(s):")
    for goal, task_type in proposals:
        print(f"\nProposed task: [{task_type}] {goal}")
        resp = input("Add to queue? [y/n]: ").strip().lower()
        if resp != "y":
            continue
        try:
            tt = TaskType(task_type)
        except ValueError:
            tt = TaskType.IMPLEMENT
        new_task = Task(
            goal=goal,
            type=tt,
            dependencies=[parent_task.id],
            proposed_by=f"agent:{parent_task.id}",
            authorized_by="human",
            status=Status.READY,
            identity=identity,
        )
        q.add(new_task)
        print(f"  Added: {new_task.id}")
        issue = linear.create_issue(new_task)
        if issue:
            q.update(new_task.id, linear_issue_id=issue["id"], linear_url=issue["url"])
            print(f"  Linear: {issue['url']}")


def _parse_proposals(text: str) -> list[tuple[str, str]]:
    """Extract '- [type] goal' lines from under '## Proposed follow-ups'."""
    proposals = []
    in_section = False
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.lower().startswith("## proposed follow-ups"):
            in_section = True
            continue
        if in_section and stripped.startswith("## "):
            break
        if in_section and stripped.startswith("-"):
            # Format: - [type] goal
            body = stripped.lstrip("- ").strip()
            if body.startswith("["):
                end = body.find("]")
                if end != -1:
                    task_type = body[1:end].strip()
                    goal = body[end + 1 :].strip()
                    if goal:
                        proposals.append((goal, task_type))
    return proposals


def pull_from_linear(identity: str):
    """Import any new Linear Todo issues labeled `agent:<identity>`.

    Imported tasks land as READY (not PENDING) — the Linear label *is* the
    human authorization to run, so no second triage step is needed.
    """
    issues = linear.fetch_todo_issues(identity)
    if not issues:
        return
    existing = {t.linear_issue_id for t in q.load() if t.linear_issue_id}
    for issue in issues:
        if issue["id"] in existing:
            continue
        new_task = Task(
            goal=issue["title"],
            type=TaskType.IMPLEMENT,
            notes=issue.get("description") or "",
            linear_issue_id=issue["id"],
            status=Status.READY,
            proposed_by="linear",
            authorized_by="linear-label",
            identity=identity,
        )
        q.add(new_task)
        print(
            f"Imported from Linear [{identity}]: "
            f"[{issue['identifier']}] {issue['title']}"
        )


def _execute_task(task: Task) -> dict:
    """Run a task through claude, persist result, update session_id on the task."""
    print(f"\nStarting: [{task.type.value}] {task.goal}")
    q.update(task.id, status=Status.IN_PROGRESS)
    if task.linear_issue_id:
        linear.update_issue_status(task.linear_issue_id, "In Progress")

    dep_paths = _dep_output_paths(task)
    result = claude_cli.run_task(task, dep_paths)

    if not result["ok"]:
        print(f"  claude error: {result['error']}")
        q.update(task.id, status=Status.BLOCKED, notes=result["error"])
        return result

    out_path = claude_cli.save_result(task.id, result)
    q.update(
        task.id,
        session_id=result["session_id"],
        output_path=out_path,
    )
    task.session_id = result["session_id"]
    task.output_path = out_path
    return result


def _run_review_loop(task: Task, result: dict) -> bool:
    """Show review gate; on reject, ask for feedback and resume claude. Returns approved bool."""
    for iteration in range(MAX_FEEDBACK_ITERATIONS):
        approved = review_gate(task, result)
        if approved:
            return True

        feedback = input("\nFeedback for Claude (empty to block task): ").strip()
        if not feedback:
            return False

        if not result.get("session_id"):
            print("  No session_id to resume — blocking task.")
            return False

        print(f"\nResuming session {result['session_id'][:8]} with feedback...")
        result = claude_cli.resume_with_feedback(result["session_id"], feedback)
        if not result["ok"]:
            print(f"  claude error on resume: {result['error']}")
            q.update(task.id, notes=f"Resume failed: {result['error']}")
            return False

        out_path = claude_cli.save_result(task.id, result)
        q.update(task.id, session_id=result["session_id"], output_path=out_path)

    print(f"  Max feedback iterations ({MAX_FEEDBACK_ITERATIONS}) reached.")
    return False


def run_loop(identity: str):
    print(f"Orchestrator running as [{identity}]. Ctrl+C to pause.")
    while True:
        pull_from_linear(identity)

        # Triage any pending proposals first (scoped to this identity)
        proposals = q.pending_proposals(identity)
        if proposals:
            print(f"\n{len(proposals)} pending proposal(s) need triage.")
            for task in proposals:
                print(f"\n[{task.type.value}] {task.goal}")
                if task.notes:
                    print(f"  Notes: {task.notes[:200]}")
                resp = input("Approve? [y/n]: ").strip().lower()
                if resp == "y":
                    q.update(task.id, status=Status.READY, authorized_by="human")
                    if not task.linear_issue_id:
                        issue = linear.create_issue(task)
                        if issue:
                            q.update(
                                task.id,
                                linear_issue_id=issue["id"],
                                linear_url=issue["url"],
                            )
                            print(f"  Linear: {issue['url']}")

        task = q.get_next_ready(identity)
        if task is None:
            print("No ready tasks. Waiting...")
            time.sleep(5)
            continue

        result = _execute_task(task)
        if not result["ok"]:
            continue

        if task.type in ALWAYS_GATE:
            approved = _run_review_loop(task, result)
            if not approved:
                q.update(task.id, status=Status.BLOCKED, notes="Human rejected output")
                continue

        q.update(task.id, status=Status.DONE)
        if task.linear_issue_id:
            linear.update_issue_status(task.linear_issue_id, "Done")

        triage_proposals(result.get("result", ""), task, identity)
        print(f"Done: {task.id}")


def seed(goal: str, task_type: str = "spec"):
    task = Task(
        goal=goal,
        type=TaskType(task_type),
        acceptance_criteria="Clear enough for an engineer to implement without asking questions",
        status=Status.READY,
        authorized_by="human",
    )
    q.add(task)
    print(f"Seeded task {task.id}: {goal}")


def main():
    parser = argparse.ArgumentParser(description="Agent work orchestrator")
    parser.add_argument("--seed", metavar="GOAL", help="Seed a task and exit")
    parser.add_argument(
        "--type", default="spec", help="Task type for --seed (default: spec)"
    )
    parser.add_argument(
        "--identity",
        metavar="NAME",
        help=(
            "Persona skill to run as (e.g. architect, programmer). "
            "Required for the execution loop. The loop will only pull "
            "Linear issues labeled `agent:<identity>`."
        ),
    )
    args = parser.parse_args()

    if args.seed:
        seed(args.seed, args.type)
        return

    if not args.identity:
        parser.error("--identity is required to run the execution loop")
    _validate_identity(args.identity)
    run_loop(args.identity)


if __name__ == "__main__":
    main()
