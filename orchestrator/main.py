"""Orchestrator loop.

Shells out to `claude -p` for task execution. Trusts .claude/settings.local.json
for tool permissions. Python owns: queue, review gates, Linear sync, feedback loops.
"""

import argparse
import logging
import os
import sys
import time
from typing import Optional

from . import claude_cli
from . import linear
from . import taskqueue as q
from .models import Status, Task, TaskType
from .review import review_gate

ALWAYS_GATE = {TaskType.SPEC, TaskType.DECIDE}
MAX_FEEDBACK_ITERATIONS = 5
SKILLS_DIR = ".claude/skills"
# How often to hit the Linear API looking for new work, in seconds.
# Linear's rate limit for personal API keys is ~1500 req/hour; 60s keeps
# us well under that even with several loops running in parallel.
LINEAR_POLL_INTERVAL_S = 60
# How long to sleep between queue checks when we're idle.
IDLE_SLEEP_S = 5

log = logging.getLogger("orchestrator.main")


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

    If a BLOCKED task already exists for a Linear issue that's back in Todo
    (typically because the human fixed whatever blocked it — granted
    permissions, edited the acceptance criteria, etc.), re-promote it to
    READY instead of importing a duplicate.
    """
    issues = linear.fetch_todo_issues(identity)
    if not issues:
        return
    all_tasks = q.load()
    by_linear_id = {t.linear_issue_id: t for t in all_tasks if t.linear_issue_id}
    for issue in issues:
        existing_task = by_linear_id.get(issue["id"])
        if existing_task is not None:
            if existing_task.status == Status.BLOCKED:
                q.update(
                    existing_task.id,
                    status=Status.READY,
                    notes=f"Re-promoted from BLOCKED: {existing_task.notes}",
                )
                print(
                    f"Re-promoted from Linear [{identity}]: "
                    f"[{issue['identifier']}] {issue['title']}"
                )
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


def _permission_denials(result: dict) -> list[dict]:
    """Extract permission_denials from a claude -p JSON result.

    Headless `claude -p` reports `is_error=false` and `stop_reason=end_turn`
    even when the LLM gave up because a tool was denied — from its perspective
    it simply ended its turn by asking a question. The real signal lives in
    `raw.permission_denials`, a list of the tool calls that were refused.
    """
    raw = result.get("raw") or {}
    return raw.get("permission_denials") or []


def _format_denials(denials: list[dict]) -> str:
    lines = []
    for d in denials:
        tool = d.get("tool_name", "?")
        path = (d.get("tool_input") or {}).get("file_path", "")
        lines.append(f"{tool}({path})" if path else tool)
    return ", ".join(lines)


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

    denials = _permission_denials(result)
    if denials:
        # Turn the ok result into a failure so the outer loop doesn't mark
        # it Done. The output is still saved so the human can see what
        # claude tried to do.
        summary = _format_denials(denials)
        msg = f"Blocked: {len(denials)} tool call(s) denied: {summary}"
        print(f"  {msg}")
        print(
            "  Add the missing tool(s) to .claude/settings.local.json "
            "`permissions.allow` and re-run."
        )
        q.update(task.id, status=Status.BLOCKED, notes=msg)
        result["ok"] = False
        result["error"] = msg
        result["denials"] = denials

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
    log.info(
        "Linear poll interval=%ss, LINEAR_API_KEY set=%s, LINEAR_TEAM_ID=%s",
        LINEAR_POLL_INTERVAL_S,
        bool(os.environ.get("LINEAR_API_KEY")),
        os.environ.get("LINEAR_TEAM_ID") or "(unset)",
    )
    last_linear_poll: Optional[float] = None
    while True:
        now = time.monotonic()
        if last_linear_poll is None or now - last_linear_poll >= LINEAR_POLL_INTERVAL_S:
            log.info("Polling Linear for new %s tasks...", identity)
            pull_from_linear(identity)
            last_linear_poll = now

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
            time.sleep(IDLE_SLEEP_S)
            continue

        result = _execute_task(task)
        if not result["ok"]:
            # If claude was blocked on permissions, walk Linear back to
            # Todo so the issue is picked up again once the permission
            # is granted. Other failure modes (subprocess errors, JSON
            # parse failures, claude binary missing) leave it in
            # In Progress for manual intervention.
            if result.get("denials") and task.linear_issue_id:
                linear.update_issue_status(task.linear_issue_id, "Todo")
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
    parser.add_argument(
        "-v", "--verbose",
        action="count",
        default=0,
        help="Increase log verbosity. -v for INFO, -vv for DEBUG.",
    )
    args = parser.parse_args()

    if args.verbose >= 2:
        level = logging.DEBUG
    elif args.verbose >= 1:
        level = logging.INFO
    else:
        level = logging.WARNING
    logging.basicConfig(
        level=level,
        format="%(asctime)s %(levelname)-5s %(name)s: %(message)s",
        datefmt="%H:%M:%S",
        force=True,
    )

    if args.seed:
        seed(args.seed, args.type)
        return

    if not args.identity:
        parser.error("--identity is required to run the execution loop")
    _validate_identity(args.identity)
    run_loop(args.identity)


if __name__ == "__main__":
    main()
