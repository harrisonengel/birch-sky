#!/usr/bin/env python3
"""
Parse PR comments to determine the latest verdict per required persona.

Only considers reviews posted after the most recent fix-iteration marker
(or after the head commit timestamp if no marker exists). This ensures
stale reviews from before the last fix are ignored.

Outputs:
  --output file: JSON with all_approved, verdicts, pending
  GITHUB_OUTPUT:  all_approved=true/false
"""

import argparse
import json
import os
import re
import sys
import urllib.request
import urllib.error
from datetime import datetime, timezone


MARKER_PATTERN = re.compile(r"<!-- fix-iteration-marker: (\d+), sha: ([0-9a-f]+) -->")
PERSONA_PATTERN = re.compile(r"<!-- persona: ([\w-]+) -->")
APPROVED_PATTERN = re.compile(r"\*\*VERDICT: APPROVED\*\*", re.IGNORECASE)
CHANGES_PATTERN = re.compile(r"\*\*VERDICT: CHANGES REQUESTED\*\*", re.IGNORECASE)

VALID_PERSONAS = {
    "architect", "decision-reviewer", "designer", "experimenter",
    "programmer", "project-manager", "quality-assurance", "security",
}


def github_get(url, token):
    req = urllib.request.Request(
        url,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def parse_iso(ts):
    """Parse ISO 8601 timestamp to datetime (UTC)."""
    return datetime.fromisoformat(ts.replace("Z", "+00:00"))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, help="owner/repo")
    parser.add_argument("--pr", required=True, help="PR number")
    parser.add_argument("--head-sha", required=True, help="Current HEAD SHA")
    parser.add_argument("--output", required=True, help="JSON output file")
    args = parser.parse_args()

    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if not token:
        print("Error: GH_TOKEN or GITHUB_TOKEN required", file=sys.stderr)
        sys.exit(1)

    base_url = f"https://api.github.com/repos/{args.repo}"

    # --- Determine required personas from PR labels ---
    pr_data = github_get(f"{base_url}/pulls/{args.pr}", token)
    labels = [l["name"] for l in pr_data.get("labels", [])]
    required_personas = [
        name[len("review:"):]
        for name in labels
        if name.startswith("review:") and name[len("review:"):] in VALID_PERSONAS
    ]

    if not required_personas:
        print("No review:* labels found — nothing required.")
        result = {"all_approved": True, "verdicts": {}, "pending": []}
        _write_output(args.output, result)
        return

    print(f"Required personas: {required_personas}")

    # --- Find the cutoff timestamp ---
    # Use the latest fix-iteration-marker comment if one exists,
    # otherwise fall back to the head commit's committer date.
    cutoff_ts = _get_cutoff_timestamp(base_url, args.pr, args.head_sha, token)
    print(f"Review cutoff timestamp: {cutoff_ts.isoformat()}")

    # --- Fetch all PR issue comments (paginated) ---
    all_comments = []
    page = 1
    while True:
        url = f"{base_url}/issues/{args.pr}/comments?per_page=100&page={page}"
        comments = github_get(url, token)
        if not comments:
            break
        all_comments.extend(comments)
        if len(comments) < 100:
            break
        page += 1

    print(f"Total comments fetched: {len(all_comments)}")

    # --- Find latest review per persona after cutoff ---
    latest_review = {}  # persona -> {created_at, body}
    for comment in all_comments:
        created_at = parse_iso(comment["created_at"])
        if created_at < cutoff_ts:
            continue
        body = comment.get("body", "")
        m = PERSONA_PATTERN.search(body)
        if not m:
            continue
        persona = m.group(1)
        if persona not in VALID_PERSONAS:
            continue
        # Keep the latest comment per persona
        if persona not in latest_review or created_at > latest_review[persona]["created_at"]:
            latest_review[persona] = {"created_at": created_at, "body": body}

    # --- Determine verdicts ---
    verdicts = {}
    pending = []
    for persona in required_personas:
        if persona not in latest_review:
            pending.append(persona)
            continue
        body = latest_review[persona]["body"]
        if APPROVED_PATTERN.search(body):
            verdicts[persona] = "APPROVED"
        elif CHANGES_PATTERN.search(body):
            verdicts[persona] = "CHANGES REQUESTED"
        else:
            # Review posted but no parseable verdict — treat as pending
            pending.append(persona)

    all_approved = (
        len(pending) == 0
        and all(v == "APPROVED" for v in verdicts.values())
        and set(verdicts.keys()) == set(required_personas)
    )

    result = {
        "all_approved": all_approved,
        "verdicts": verdicts,
        "pending": pending,
    }

    print(f"Verdicts: {verdicts}")
    print(f"Pending (no current review): {pending}")
    print(f"All approved: {all_approved}")

    _write_output(args.output, result)

    # Write to GITHUB_OUTPUT for workflow condition checks
    github_output = os.environ.get("GITHUB_OUTPUT")
    if github_output:
        with open(github_output, "a") as f:
            f.write(f"all_approved={'true' if all_approved else 'false'}\n")
            f.write(f"pending_count={len(pending)}\n")
            changes_requested = [p for p, v in verdicts.items() if v == "CHANGES REQUESTED"]
            f.write(f"changes_requested={json.dumps(changes_requested)}\n")


def _get_cutoff_timestamp(base_url, pr_number, head_sha, token):
    """
    Return the cutoff datetime: latest fix-iteration-marker comment timestamp,
    or the head commit's committer date if no marker exists.
    """
    # Check for fix-iteration-marker comments
    page = 1
    latest_marker_ts = None
    while True:
        url = f"{base_url}/issues/{pr_number}/comments?per_page=100&page={page}"
        comments = github_get(url, token)
        if not comments:
            break
        for comment in comments:
            body = comment.get("body", "")
            if MARKER_PATTERN.search(body):
                ts = parse_iso(comment["created_at"])
                if latest_marker_ts is None or ts > latest_marker_ts:
                    latest_marker_ts = ts
        if len(comments) < 100:
            break
        page += 1

    if latest_marker_ts is not None:
        return latest_marker_ts

    # Fall back to head commit timestamp
    commit_data = github_get(f"{base_url}/commits/{head_sha}", token)
    committer_date = commit_data["commit"]["committer"]["date"]
    return parse_iso(committer_date)


def _write_output(path, result):
    with open(path, "w") as f:
        json.dump(result, f, indent=2, default=str)
    print(f"Results written to {path}")


if __name__ == "__main__":
    main()
