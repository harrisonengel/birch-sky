"""Orchestrator loop.

Shells out to `claude -p` for task execution. Trusts .claude/settings.local.json
for tool permissions. Python owns: queue, review gates, Linear sync, feedback loops.
"""

import argparse
import logging
import os
import re
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

APPROVAL_KEYWORDS = {"approved", "approve", "lgtm", "yes", "ship it", "ship"}

log = logging.getLogger("orchestrator.main")


def _format_approval_comment(task: Task, result: dict) -> str:
    """Format the review comment posted to a Linear issue when approval is needed."""
    lines = [
        "## 🔒 Agent needs your approval\n",
        f"**Task:** [{task.type.value}] {task.goal}\n",
        f"**Session:** `{result.get('session_id', '?')[:8]}`\n",
        "---\n",
        "### Output summary\n",
    ]

    message = result.get("result", "") or "(no output)"
    msg_lines = message.splitlines()
    lines.append("\n".join(msg_lines[:80]))
    if len(msg_lines) > 80:
        lines.append(f"\n\n*… ({len(msg_lines) - 80} more lines — see full output in repo)*")

    # Include proposed follow-ups if any
    proposals = _parse_proposals(message)
    if proposals:
        lines.append("\n\n### Proposed follow-ups\n")
        for i, (goal, task_type) in enumerate(proposals, 1):
            lines.append(f"{i}. **[{task_type}]** {goal}")

    lines.append("\n\n---")
    lines.append("### How to respond")
    lines.append(
        "Reply with **approved** (or lgtm / yes / ship it) to accept, "
        "or write feedback to request changes."
    )
    if proposals:
        lines.append(
            "To approve follow-ups, include which ones "
            '(e.g. "approved. follow up on 1 and 3").'
        )
    lines.append("\nThen **move this ticket back to Todo**.")

    return "\n".join(lines)


def _request_linear_approval(task: Task, result: dict) -> None:
    """Post approval request to Linear, move to Blocked, assign to approver."""
    comment_body = _format_approval_comment(task, result)
    comment_id = linear.add_comment(task.linear_issue_id, comment_body)

    # Move to Blocked (fall back to In Review)
    moved = linear.update_issue_status(task.linear_issue_id, "Blocked")
    if not moved:
        linear.update_issue_status(task.linear_issue_id, "In Review")

    # Assign to the configured approver
    assignee_id = os.environ.get("LINEAR_APPROVAL_ASSIGNEE_ID")
    if assignee_id:
        linear.assign_issue(task.linear_issue_id, assignee_id)

    q.update(
        task.id,
        status=Status.REVIEW,
        notes=f"Awaiting Linear approval (comment={comment_id})",
    )
    print(f"  Posted approval request to Linear. Waiting for response.")


def _is_approval(text: str) -> bool:
    """Check if a response is an approval keyword."""
    normalized = text.strip().lower().rstrip(".!,")
    # Check the first line only — rest may be proposal selections
    first_line = normalized.split("\n")[0].strip()
    return first_line in APPROVAL_KEYWORDS


def _parse_proposal_selections(text: str) -> list[int]:
    """Parse which proposals the human approved from their response.

    Looks for patterns like 'follow up on 1 and 3', 'approve 1, 2',
    'proposals: 1 3', etc. Returns 1-indexed proposal numbers.
    """
    numbers = re.findall(r"\b(\d+)\b", text)
    return [int(n) for n in numbers]


def _handle_linear_approval_response(
    task: Task,
    identity: str,
    linear_approval: bool,
) -> None:
    """Process a human's Linear response for a REVIEW task that's back in Todo."""
    comments = linear.fetch_issue_comments(task.linear_issue_id)
    if not comments:
        log.warning("No comments on issue for task %s — re-promoting as ready", task.id)
        q.update(task.id, status=Status.READY, notes="No response found, retrying")
        return

    # The human's response is the latest comment
    response = comments[-1]
    response_text = response.get("body", "").strip()
    log.info(
        "Approval response for %s from %s: %s",
        task.id,
        (response.get("user") or {}).get("name", "?"),
        response_text[:200],
    )

    if not response_text:
        log.warning("Empty response for task %s — re-promoting", task.id)
        q.update(task.id, status=Status.READY, notes="Empty response, retrying")
        return

    if _is_approval(response_text):
        # Approved — mark done
        print(f"  Approved via Linear: {task.goal}")
        q.update(task.id, status=Status.DONE)
        linear.update_issue_status(task.linear_issue_id, "Done")

        # Handle proposal selections from the approval response
        result_text = ""
        if task.output_path and os.path.exists(task.output_path):
            with open(task.output_path) as f:
                result_text = f.read()
        proposals = _parse_proposals(result_text)
        if proposals:
            selections = _parse_proposal_selections(response_text)
            if selections:
                _create_selected_proposals(proposals, selections, task, identity)
            else:
                # "approved" with no numbers — approve all proposals
                _create_selected_proposals(
                    proposals, list(range(1, len(proposals) + 1)), task, identity
                )
        return

    # Not an approval — treat as feedback, resume session
    if not task.session_id:
        print(f"  No session to resume for {task.id} — blocking.")
        q.update(task.id, status=Status.BLOCKED, notes="Feedback received but no session to resume")
        return

    print(f"  Resuming {task.session_id[:8]} with Linear feedback...")
    result = claude_cli.resume_with_feedback(task.session_id, response_text)
    if not result["ok"]:
        print(f"  Resume failed: {result['error']}")
        q.update(task.id, status=Status.BLOCKED, notes=f"Resume failed: {result['error']}")
        return

    out_path = claude_cli.save_result(task.id, result)
    q.update(task.id, session_id=result["session_id"], output_path=out_path)
    task.session_id = result["session_id"]
    task.output_path = out_path

    # Re-post for another round of approval
    if task.linear_issue_id:
        _request_linear_approval(task, result)
    else:
        # No Linear issue — fall back to blocking
        q.update(task.id, status=Status.BLOCKED, notes="Feedback applied but no Linear issue for re-approval")


def _create_selected_proposals(
    proposals: list[tuple[str, str]],
    selections: list[int],
    parent_task: Task,
    identity: str,
) -> None:
    """Create tasks for the selected proposals (1-indexed)."""
    for idx in selections:
        if idx < 1 or idx > len(proposals):
            continue
        goal, task_type = proposals[idx - 1]
        try:
            tt = TaskType(task_type)
        except ValueError:
            tt = TaskType.IMPLEMENT
        new_task = Task(
            goal=goal,
            type=tt,
            dependencies=[parent_task.id],
            proposed_by=f"agent:{parent_task.id}",
            authorized_by="human-linear",
            status=Status.READY,
            identity=identity,
        )
        q.add(new_task)
        print(f"  Approved proposal: {new_task.id} — {goal}")
        issue = linear.create_issue(new_task)
        if issue:
            q.update(new_task.id, linear_issue_id=issue["id"], linear_url=issue["url"])
            print(f"  Linear: {issue['url']}")


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


def pull_from_linear(identity: str, *, linear_approval: bool = False):
    """Import any new Linear Todo issues labeled `agent:<identity>`.

    Imported tasks land as READY (not PENDING) — the Linear label *is* the
    human authorization to run, so no second triage step is needed.

    If a BLOCKED task already exists for a Linear issue that's back in Todo
    (typically because the human fixed whatever blocked it — granted
    permissions, edited the acceptance criteria, etc.), re-promote it to
    READY instead of importing a duplicate.

    If a REVIEW task's issue is back in Todo, the human has responded to an
    approval request. Read their response and handle accordingly.
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
            elif existing_task.status == Status.REVIEW and linear_approval:
                print(
                    f"Approval response from Linear [{identity}]: "
                    f"[{issue['identifier']}] {issue['title']}"
                )
                _handle_linear_approval_response(existing_task, identity, linear_approval)
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


def run_loop(identity: str, *, linear_approval: bool = False):
    mode = "Linear-approval" if linear_approval else "CLI"
    print(f"Orchestrator running as [{identity}] ({mode} mode). Ctrl+C to pause.")
    log.info(
        "Linear poll interval=%ss, LINEAR_API_KEY set=%s, LINEAR_TEAM_ID=%s, linear_approval=%s",
        LINEAR_POLL_INTERVAL_S,
        bool(os.environ.get("LINEAR_API_KEY")),
        os.environ.get("LINEAR_TEAM_ID") or "(unset)",
        linear_approval,
    )
    if linear_approval and not os.environ.get("LINEAR_APPROVAL_ASSIGNEE_ID"):
        log.warning(
            "LINEAR_APPROVAL_ASSIGNEE_ID not set — approval tickets won't be auto-assigned."
        )
    last_linear_poll: Optional[float] = None
    while True:
        now = time.monotonic()
        if last_linear_poll is None or now - last_linear_poll >= LINEAR_POLL_INTERVAL_S:
            log.info("Polling Linear for new %s tasks...", identity)
            pull_from_linear(identity, linear_approval=linear_approval)
            last_linear_poll = now

        # Triage any pending proposals first (scoped to this identity)
        if not linear_approval:
            # In CLI mode, triage interactively as before
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
            if linear_approval and task.linear_issue_id:
                # Async approval via Linear — don't block
                _request_linear_approval(task, result)
                continue
            else:
                # CLI approval — blocks on input()
                approved = _run_review_loop(task, result)
                if not approved:
                    q.update(task.id, status=Status.BLOCKED, notes="Human rejected output")
                    continue

        q.update(task.id, status=Status.DONE)
        if task.linear_issue_id:
            linear.update_issue_status(task.linear_issue_id, "Done")

        if linear_approval and task.linear_issue_id:
            # In linear-approval mode, proposals are handled via the
            # approval comment (included by _format_approval_comment for
            # ALWAYS_GATE tasks) or auto-queued for non-gated tasks.
            proposals = _parse_proposals(result.get("result", ""))
            if proposals:
                _create_selected_proposals(
                    proposals, list(range(1, len(proposals) + 1)), task, identity
                )
        else:
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
        "--linear-approval",
        action="store_true",
        default=False,
        help=(
            "Use Linear for approval instead of CLI input(). "
            "When a task needs review, the orchestrator posts reasoning "
            "to the Linear ticket, moves it to Blocked, and assigns it to "
            "LINEAR_APPROVAL_ASSIGNEE_ID. Moving the ticket back to Todo "
            "with a response comment continues the loop."
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
    run_loop(args.identity, linear_approval=args.linear_approval)


if __name__ == "__main__":
    main()
