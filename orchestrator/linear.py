"""Linear integration for the orchestrator.

Uses stdlib urllib (no new dependencies). Linear is a *view* on top of the
local backlog.json — backlog.json is the source of truth. If they diverge,
the queue wins and Linear gets reconciled.

Required environment:
    LINEAR_API_KEY    Personal API key from Linear → Settings → API
    LINEAR_TEAM_ID    Team ID from Linear → Settings → Teams
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from typing import Optional

from .models import Task

LINEAR_API = "https://api.linear.app/graphql"

_state_cache: dict = {}


def _enabled() -> bool:
    return bool(os.environ.get("LINEAR_API_KEY") and os.environ.get("LINEAR_TEAM_ID"))


def _gql(query: str, variables: Optional[dict] = None) -> dict:
    api_key = os.environ.get("LINEAR_API_KEY")
    if not api_key:
        raise RuntimeError("LINEAR_API_KEY not set")

    body = json.dumps({"query": query, "variables": variables or {}}).encode()
    req = urllib.request.Request(
        LINEAR_API,
        data=body,
        headers={"Authorization": api_key, "Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req) as resp:
            data = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"Linear API error {e.code}: {e.read().decode()}") from e

    if "errors" in data:
        raise RuntimeError(f"Linear GraphQL error: {data['errors']}")
    return data


_GET_STATES = """
query($teamId: String!) {
  workflowStates(filter: { team: { id: { eq: $teamId } } }) {
    nodes { id name }
  }
}
"""


def _get_state_id(name: str) -> Optional[str]:
    """Cached lookup of workflow state ID by name (e.g. 'Todo', 'In Progress', 'Done')."""
    if not _state_cache:
        team_id = os.environ["LINEAR_TEAM_ID"]
        data = _gql(_GET_STATES, {"teamId": team_id})
        for s in data["data"]["workflowStates"]["nodes"]:
            _state_cache[s["name"]] = s["id"]
    return _state_cache.get(name)


_CREATE_ISSUE = """
mutation($title: String!, $description: String!, $teamId: String!, $stateId: String) {
  issueCreate(input: {
    title: $title,
    description: $description,
    teamId: $teamId,
    stateId: $stateId
  }) {
    success
    issue { id identifier url }
  }
}
"""


def create_issue(task: Task, state_name: str = "Todo") -> Optional[dict]:
    """Create a Linear issue from a task. Returns {id, identifier, url} or None on failure."""
    if not _enabled():
        return None

    description = (
        f"**Goal:** {task.goal}\n\n"
        f"**Type:** {task.type.value}\n\n"
        f"**Acceptance criteria:** {task.acceptance_criteria or '(none)'}\n\n"
        f"**Orchestrator task ID:** `{task.id}`\n"
        f"**Dependencies:** {', '.join(task.dependencies) or 'none'}"
    )

    try:
        data = _gql(
            _CREATE_ISSUE,
            {
                "title": task.goal[:100],
                "description": description,
                "teamId": os.environ["LINEAR_TEAM_ID"],
                "stateId": _get_state_id(state_name),
            },
        )
        return data["data"]["issueCreate"]["issue"]
    except Exception as e:
        print(f"  Linear create_issue failed: {e}", file=sys.stderr)
        return None


_UPDATE_ISSUE = """
mutation($id: String!, $stateId: String!) {
  issueUpdate(id: $id, input: { stateId: $stateId }) {
    success
  }
}
"""


def update_issue_status(issue_id: str, state_name: str) -> bool:
    """Move a Linear issue to a new workflow state by name."""
    if not _enabled() or not issue_id:
        return False
    try:
        state_id = _get_state_id(state_name)
        if not state_id:
            print(f"  Linear: unknown state '{state_name}'", file=sys.stderr)
            return False
        data = _gql(_UPDATE_ISSUE, {"id": issue_id, "stateId": state_id})
        return data["data"]["issueUpdate"]["success"]
    except Exception as e:
        print(f"  Linear update_issue_status failed: {e}", file=sys.stderr)
        return False


_GET_TODO_ISSUES = """
query($teamId: String!) {
  issues(filter: {
    team: { id: { eq: $teamId } },
    state: { name: { eq: "Todo" } }
  }) {
    nodes { id identifier title description }
  }
}
"""


def fetch_todo_issues() -> list[dict]:
    """Pull issues currently in the Todo column. Returns [] if Linear disabled."""
    if not _enabled():
        return []
    try:
        data = _gql(_GET_TODO_ISSUES, {"teamId": os.environ["LINEAR_TEAM_ID"]})
        return data["data"]["issues"]["nodes"]
    except Exception as e:
        print(f"  Linear fetch_todo_issues failed: {e}", file=sys.stderr)
        return []
