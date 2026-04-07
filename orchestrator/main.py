import argparse
import time

from .models import Task, TaskType, Status
from .context import build
from .agent import run
from . import taskqueue as q
from . import linear
from .review import review_gate

ALWAYS_GATE = {TaskType.SPEC, TaskType.DECIDE}


def triage_proposals(result, parent_task: Task):
    for proposal in result.proposed_tasks:
        print(f"\nProposed task: [{proposal.get('type', '?')}] {proposal.get('goal', '?')}")
        resp = input("Add to queue? [y/n]: ").strip().lower()
        if resp == "y":
            new_task = Task(
                goal=proposal["goal"],
                type=TaskType(proposal.get("type", "implement")),
                acceptance_criteria=proposal.get("acceptance_criteria", ""),
                dependencies=[parent_task.id],
                proposed_by=f"agent:{parent_task.id}",
                authorized_by="human",
                status=Status.READY,
            )
            q.add(new_task)
            print(f"  Added: {new_task.id}")
            issue = linear.create_issue(new_task)
            if issue:
                q.update(new_task.id, linear_issue_id=issue["id"], linear_url=issue["url"])
                print(f"  Linear: {issue['url']}")


def pull_from_linear():
    """Import any new Linear Todo issues that aren't yet in the queue."""
    issues = linear.fetch_todo_issues()
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
            status=Status.PENDING,  # still needs human authorization
            proposed_by="linear",
        )
        q.add(new_task)
        print(f"Imported from Linear: [{issue['identifier']}] {issue['title']}")


def run_loop():
    print("Orchestrator running. Ctrl+C to pause.")
    while True:
        pull_from_linear()

        # Triage any pending proposals first
        proposals = q.pending_proposals()
        if proposals:
            print(f"\n{len(proposals)} pending proposal(s) need triage.")
            for task in proposals:
                print(f"\n[{task.type}] {task.goal}")
                resp = input("Approve? [y/n]: ").strip().lower()
                if resp == "y":
                    q.update(task.id, status=Status.READY, authorized_by="human")
                    if not task.linear_issue_id:
                        issue = linear.create_issue(task)
                        if issue:
                            q.update(task.id, linear_issue_id=issue["id"], linear_url=issue["url"])
                            print(f"  Linear: {issue['url']}")

        task = q.get_next_ready()
        if task is None:
            print("No ready tasks. Waiting...")
            time.sleep(5)
            continue

        print(f"\nStarting: [{task.type}] {task.goal}")
        q.update(task.id, status=Status.IN_PROGRESS)
        if task.linear_issue_id:
            linear.update_issue_status(task.linear_issue_id, "In Progress")

        system_prompt = build(task)
        result = run(task, system_prompt)

        # Check for API errors
        if result.output.startswith("API ERROR:"):
            print(f"Task {task.id} failed: {result.output}")
            q.update(task.id, status=Status.BLOCKED, notes=result.output)
            continue

        needs_gate = task.type in ALWAYS_GATE
        if needs_gate:
            approved = review_gate(task, result)
            if not approved:
                q.update(task.id, status=Status.BLOCKED, notes="Human rejected output")
                continue

        q.update(task.id, status=Status.DONE, output_path=f"outputs/{task.id}/output.md")
        if task.linear_issue_id:
            linear.update_issue_status(task.linear_issue_id, "Done")
        triage_proposals(result, task)
        print(f"Done: {task.id}")


def seed(goal: str, task_type: str = "spec"):
    task = Task(
        goal=goal,
        type=TaskType(task_type),
        context_refs=["CLAUDE.md"],
        acceptance_criteria="Clear enough for an engineer to implement without asking questions",
        status=Status.READY,
        authorized_by="human",
    )
    q.add(task)
    print(f"Seeded task {task.id}: {goal}")


def main():
    parser = argparse.ArgumentParser(description="Agent work orchestrator")
    parser.add_argument("--seed", metavar="GOAL", help="Seed a task and exit")
    parser.add_argument("--type", default="spec", help="Task type for --seed (default: spec)")
    args = parser.parse_args()

    if args.seed:
        seed(args.seed, args.type)
    else:
        run_loop()


if __name__ == "__main__":
    main()
