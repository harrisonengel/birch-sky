#!/usr/bin/env python3
"""
Collect review feedback for personas that requested changes.

Reads the verdict JSON, fetches the relevant PR comments from GitHub,
and writes the feedback as markdown to --output. The output is empty
if no feedback is found (so the caller can detect the no-op case).

Cutoff logic: only considers comments posted after the most recent
fix-iteration marker, or after the head-commit timestamp if none exists.
This mirrors check_reviews.py so stale pre-fix reviews are ignored.
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
    return datetime.fromisoformat(ts.replace("Z", "+00:00"))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, help="owner/repo")
    parser.add_argument("--pr", required=True, help="PR number")
    parser.add_argument("--head-sha", required=True, help="Current HEAD SHA")
    parser.add_argument("--verdict-file", required=True, help="JSON from check_reviews.py")
    parser.add_argument("--output", required=True, help="File to write feedback markdown to")
    args = parser.parse_args()

    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if not token:
        print("Error: GH_TOKEN or GITHUB_TOKEN required", file=sys.stderr)
        sys.exit(1)

    with open(args.verdict_file) as f:
        verdict_data = json.load(f)

    changes_requested = {
        p for p, v in verdict_data.get("verdicts", {}).items()
        if v == "CHANGES REQUESTED"
    }

    if not changes_requested:
        open(args.output, "w").close()
        return

    base_url = f"https://api.github.com/repos/{args.repo}"

    # Determine cutoff: latest fix-iteration marker, or head commit timestamp
    page = 1
    latest_marker_ts = None
    while True:
        url = f"{base_url}/issues/{args.pr}/comments?per_page=100&page={page}"
        comments = github_get(url, token)
        if not comments:
            break
        for comment in comments:
            if MARKER_PATTERN.search(comment.get("body", "")):
                ts = parse_iso(comment["created_at"])
                if latest_marker_ts is None or ts > latest_marker_ts:
                    latest_marker_ts = ts
        if len(comments) < 100:
            break
        page += 1

    if latest_marker_ts is not None:
        cutoff = latest_marker_ts
    else:
        commit_data = github_get(f"{base_url}/commits/{args.head_sha}", token)
        cutoff = parse_iso(commit_data["commit"]["committer"]["date"])

    # Collect all comments, find the latest review per persona after the cutoff
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

    latest = {}
    for comment in all_comments:
        created_at = parse_iso(comment["created_at"])
        if created_at < cutoff:
            continue
        body = comment.get("body", "")
        m = PERSONA_PATTERN.search(body)
        if not m:
            continue
        persona = m.group(1)
        if persona not in changes_requested:
            continue
        if persona not in latest or created_at > latest[persona]["ts"]:
            latest[persona] = {"ts": created_at, "body": body}

    feedback_parts = [
        f"### Review from: {persona}\n\n{data['body']}"
        for persona, data in latest.items()
    ]

    with open(args.output, "w") as f:
        f.write("\n\n---\n\n".join(feedback_parts))

    if feedback_parts:
        print(f"Collected feedback from: {', '.join(latest)}")
    else:
        print("No matching feedback found after cutoff.")


if __name__ == "__main__":
    main()
