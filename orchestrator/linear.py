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
import logging
import os
import sys
import urllib.error
import urllib.request
from typing import Optional

from .models import Task

LINEAR_API = "https://api.linear.app/graphql"

log = logging.getLogger("orchestrator.linear")

_state_cache: dict = {}


def _enabled() -> bool:
    return bool(os.environ.get("LINEAR_API_KEY") and os.environ.get("LINEAR_TEAM_ID"))


def _gql(query: str, variables: Optional[dict] = None) -> dict:
    api_key = os.environ.get("LINEAR_API_KEY")
    if not api_key:
        raise RuntimeError("LINEAR_API_KEY not set")

    log.debug("Linear GraphQL request variables=%s", variables)
    log.debug("Linear GraphQL query:\n%s", query.strip())

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
    log.debug("Linear GraphQL response: %s", json.dumps(data, indent=2)[:2000])
    return data


_GET_STATES = """
query($teamId: ID!) {
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
query($teamId: ID!, $label: String!) {
  issues(filter: {
    team: { id: { eq: $teamId } },
    state: { name: { eq: "Todo" } },
    labels: { name: { eq: $label } }
  }) {
    nodes { id identifier title description state { name } labels { nodes { name } } }
  }
}
"""

_DIAGNOSE_ISSUES = """
query($teamId: ID!) {
  issues(filter: { team: { id: { eq: $teamId } } }, first: 50) {
    nodes { identifier title state { name } labels { nodes { name } } }
  }
}
"""


def fetch_todo_issues(identity: str) -> list[dict]:
    """Pull Todo issues labeled `agent:<identity>`. Returns [] if Linear disabled."""
    if not _enabled():
        log.debug("Linear disabled (LINEAR_API_KEY or LINEAR_TEAM_ID unset)")
        return []
    label = f"agent:{identity}"
    log.debug("Fetching Todo issues with label=%s", label)
    try:
        data = _gql(
            _GET_TODO_ISSUES,
            {
                "teamId": os.environ["LINEAR_TEAM_ID"],
                "label": label,
            },
        )
        nodes = data["data"]["issues"]["nodes"]
        log.info("Linear returned %d issue(s) matching state=Todo label=%s", len(nodes), label)
        for n in nodes:
            log.debug(
                "  [%s] %s (state=%s labels=%s)",
                n.get("identifier"),
                n.get("title"),
                (n.get("state") or {}).get("name"),
                [l.get("name") for l in (n.get("labels") or {}).get("nodes", [])],
            )
        if not nodes and log.isEnabledFor(logging.INFO):
            _diagnose_missing(label)
        return nodes
    except Exception as e:
        print(f"  Linear fetch_todo_issues failed: {e}", file=sys.stderr)
        return []


def _diagnose_missing(expected_label: str) -> None:
    """When the filtered query returned nothing, dump a snapshot of the team's
    issues so the user can see *why* — wrong state name, wrong label, or
    nothing labeled at all."""
    try:
        data = _gql(_DIAGNOSE_ISSUES, {"teamId": os.environ["LINEAR_TEAM_ID"]})
    except Exception as e:
        log.info("Diagnostic dump failed: %s", e)
        return
    nodes = data["data"]["issues"]["nodes"]
    if not nodes:
        log.info("Diagnostic: team has no issues at all.")
        return
    log.info(
        "Diagnostic: filter matched 0 issues. Snapshot of up to %d team issues "
        "(looking for state=Todo label=%s):",
        len(nodes),
        expected_label,
    )
    for n in nodes:
        state = (n.get("state") or {}).get("name")
        labels = [l.get("name") for l in (n.get("labels") or {}).get("nodes", [])]
        marker = ""
        if state == "Todo" and expected_label in labels:
            marker = " <-- SHOULD MATCH"
        elif expected_label in labels:
            marker = f" <-- has label but state is {state!r}, not 'Todo'"
        elif state == "Todo":
            marker = f" <-- in Todo but labels={labels}"
        log.info("  [%s] %s  state=%r labels=%s%s",
                 n.get("identifier"), n.get("title"), state, labels, marker)
