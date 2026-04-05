#!/usr/bin/env python3
"""
Apply code fixes based on persona review feedback using Claude with write_file tool use.

Steps:
  1. Read verdict JSON to find personas with CHANGES REQUESTED
  2. Fetch those personas' latest review comments from GitHub API
  3. Get the PR diff to identify changed files; read their current content
  4. Call Claude API with write_file tool — multi-turn until stop_reason != tool_use
  5. Apply file writes (with path safety checks)
  6. git add + git commit
"""

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.request
import urllib.error
from datetime import datetime, timezone

MARKER_PATTERN = re.compile(r"<!-- fix-iteration-marker: (\d+), sha: ([0-9a-f]+) -->")
PERSONA_PATTERN = re.compile(r"<!-- persona: ([\w-]+) -->")
CHANGES_PATTERN = re.compile(r"\*\*VERDICT: CHANGES REQUESTED\*\*", re.IGNORECASE)

MAX_FILE_CHARS = 40_000
MAX_TOTAL_CHARS = 120_000

ALLOWED_EXTENSIONS = {
    ".py", ".ts", ".js", ".tsx", ".jsx", ".go", ".java", ".rb", ".rs",
    ".md", ".yml", ".yaml", ".json", ".toml", ".cfg", ".ini",
    ".html", ".css", ".scss", ".sql", ".sh", ".txt",
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


def call_anthropic(api_key, request_body):
    data = json.dumps(request_body).encode()
    req = urllib.request.Request(
        "https://api.anthropic.com/v1/messages",
        data=data,
        headers={
            "Content-Type": "application/json",
            "x-api-key": api_key,
            "anthropic-version": "2023-06-01",
        },
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        print(f"Anthropic API error ({e.code}): {body}", file=sys.stderr)
        sys.exit(1)


def safe_write(repo_root, relative_path, content):
    """Write file, enforcing path safety."""
    # Reject obvious traversal attempts early
    if relative_path.startswith("/") or ".." in relative_path.split("/"):
        raise ValueError(f"Unsafe path rejected: {relative_path}")

    full_path = os.path.realpath(os.path.join(repo_root, relative_path))
    real_root = os.path.realpath(repo_root)

    if not full_path.startswith(real_root + os.sep) and full_path != real_root:
        raise ValueError(f"Path traversal rejected: {relative_path}")

    ext = os.path.splitext(full_path)[1].lower()
    if ext and ext not in ALLOWED_EXTENSIONS:
        raise ValueError(f"Extension not allowed: {ext}")

    os.makedirs(os.path.dirname(full_path), exist_ok=True)
    with open(full_path, "w") as f:
        f.write(content)
    print(f"  Wrote: {relative_path}")
    return full_path


def parse_iso(ts):
    return datetime.fromisoformat(ts.replace("Z", "+00:00"))


def get_cutoff_timestamp(base_url, pr_number, head_sha, token):
    """Same logic as check_reviews.py — find latest marker or fall back to commit timestamp."""
    page = 1
    latest_marker_ts = None
    while True:
        url = f"{base_url}/issues/{pr_number}/comments?per_page=100&page={page}"
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
        return latest_marker_ts

    commit_data = github_get(f"{base_url}/commits/{head_sha}", token)
    return parse_iso(commit_data["commit"]["committer"]["date"])


def collect_review_feedback(base_url, pr_number, head_sha, changes_requested_personas, token):
    """Collect the review text for all personas that requested changes."""
    cutoff = get_cutoff_timestamp(base_url, pr_number, head_sha, token)

    # Fetch all comments
    all_comments = []
    page = 1
    while True:
        url = f"{base_url}/issues/{pr_number}/comments?per_page=100&page={page}"
        comments = github_get(url, token)
        if not comments:
            break
        all_comments.extend(comments)
        if len(comments) < 100:
            break
        page += 1

    # Find latest review per persona
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
        if persona not in changes_requested_personas:
            continue
        if persona not in latest or created_at > latest[persona]["ts"]:
            latest[persona] = {"ts": created_at, "body": body}

    feedback_parts = []
    for persona, data in latest.items():
        feedback_parts.append(f"### Review from: {persona}\n\n{data['body']}")

    return "\n\n---\n\n".join(feedback_parts)


def get_changed_files(repo_root):
    """Return list of files changed in the current branch vs origin base."""
    result = subprocess.run(
        ["git", "diff", "--name-only", "origin/HEAD...HEAD"],
        cwd=repo_root,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        # Fallback: all tracked files that differ from HEAD~1
        result = subprocess.run(
            ["git", "diff", "--name-only", "HEAD~1", "HEAD"],
            cwd=repo_root,
            capture_output=True,
            text=True,
        )
    files = [f.strip() for f in result.stdout.splitlines() if f.strip()]
    return files


def read_files_for_context(repo_root, changed_files):
    """Read changed files up to the total char budget."""
    sections = []
    total = 0
    for rel_path in changed_files:
        full_path = os.path.join(repo_root, rel_path)
        if not os.path.isfile(full_path):
            continue
        try:
            with open(full_path) as f:
                content = f.read(MAX_FILE_CHARS)
        except (UnicodeDecodeError, OSError):
            continue
        if total + len(content) > MAX_TOTAL_CHARS:
            remaining = MAX_TOTAL_CHARS - total
            if remaining <= 0:
                break
            content = content[:remaining] + "\n[... truncated ...]"
        sections.append(f"### File: {rel_path}\n```\n{content}\n```")
        total += len(content)
        if total >= MAX_TOTAL_CHARS:
            break
    return "\n\n".join(sections)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--pr", required=True)
    parser.add_argument("--verdict-file", required=True)
    parser.add_argument("--head-sha", required=True)
    args = parser.parse_args()

    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    api_key = os.environ.get("ANTHROPIC_API_KEY")

    if not token:
        print("Error: GH_TOKEN or GITHUB_TOKEN required", file=sys.stderr)
        sys.exit(1)
    if not api_key:
        print("Error: ANTHROPIC_API_KEY required", file=sys.stderr)
        sys.exit(1)

    repo_root = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True, text=True,
    ).stdout.strip()

    # Load verdicts
    with open(args.verdict_file) as f:
        verdict_data = json.load(f)

    changes_requested = [
        p for p, v in verdict_data.get("verdicts", {}).items()
        if v == "CHANGES REQUESTED"
    ]

    if not changes_requested:
        print("No CHANGES REQUESTED verdicts — nothing to fix.")
        sys.exit(0)

    print(f"Fixing issues from: {changes_requested}")

    base_url = f"https://api.github.com/repos/{args.repo}"

    # Collect review feedback
    feedback = collect_review_feedback(
        base_url, args.pr, args.head_sha, changes_requested, token
    )
    if not feedback:
        print("Could not find review comments — aborting.", file=sys.stderr)
        sys.exit(1)

    # Read current state of changed files
    changed_files = get_changed_files(repo_root)
    print(f"Changed files: {changed_files}")
    file_context = read_files_for_context(repo_root, changed_files)

    # Build prompt
    system_prompt = """You are a senior software engineer applying code review feedback.
Your task is to fix the issues identified in the persona reviews below.
Use the write_file tool to make changes. Only fix issues that are clearly identified
in the reviews. Do not refactor unrelated code or add features beyond what is asked.
Make the minimum changes necessary to address the feedback."""

    user_message = f"""Please fix the issues identified in these persona reviews.

## Review Feedback (CHANGES REQUESTED)

{feedback}

## Current File Contents

{file_context}

Use the write_file tool to apply your fixes. Write the complete updated content for each file you change."""

    tools = [
        {
            "name": "write_file",
            "description": (
                "Write content to a file in the repository. "
                "Path must be relative to the repository root (e.g., 'src/foo.py'). "
                "Write the complete file content, not just a diff."
            ),
            "input_schema": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Relative path from repo root",
                    },
                    "content": {
                        "type": "string",
                        "description": "Complete file content to write",
                    },
                },
                "required": ["path", "content"],
            },
        }
    ]

    # Multi-turn tool use loop
    messages = [{"role": "user", "content": user_message}]
    written_files = []

    print("Calling Claude to generate fixes...")
    for _turn in range(10):  # safety cap on turns
        response = call_anthropic(api_key, {
            "model": "claude-sonnet-4-20250514",
            "max_tokens": 4096,
            "system": system_prompt,
            "tools": tools,
            "messages": messages,
        })

        tool_results = []
        for block in response.get("content", []):
            if block.get("type") == "tool_use" and block.get("name") == "write_file":
                path = block["input"].get("path", "")
                content = block["input"].get("content", "")
                try:
                    full_path = safe_write(repo_root, path, content)
                    written_files.append(path)
                    tool_results.append({
                        "type": "tool_result",
                        "tool_use_id": block["id"],
                        "content": "OK",
                    })
                except ValueError as e:
                    print(f"  Skipped unsafe write: {e}", file=sys.stderr)
                    tool_results.append({
                        "type": "tool_result",
                        "tool_use_id": block["id"],
                        "content": f"Error: {e}",
                        "is_error": True,
                    })

        if response.get("stop_reason") != "tool_use":
            break

        messages.append({"role": "assistant", "content": response["content"]})
        messages.append({"role": "user", "content": tool_results})

    if not written_files:
        print("Fix agent made no file changes.")
        sys.exit(2)  # Non-zero so fix-loop.yml can detect and post a comment

    print(f"Files written: {written_files}")

    # Stage and commit
    subprocess.run(["git", "add"] + written_files, cwd=repo_root, check=True)

    # Check if anything is actually staged
    staged = subprocess.run(
        ["git", "diff", "--staged", "--quiet"],
        cwd=repo_root,
    )
    if staged.returncode == 0:
        print("No staged changes after write — files were already up to date.")
        sys.exit(2)

    # Determine iteration number from commit message
    count_result = subprocess.run(
        ["git", "log", "--oneline", "--author=github-actions[bot]", "HEAD"],
        cwd=repo_root,
        capture_output=True,
        text=True,
    )
    iteration = sum(
        1 for line in count_result.stdout.splitlines()
        if "fix: apply persona review feedback" in line
    ) + 1

    commit_msg = f"fix: apply persona review feedback (iteration {iteration})"
    subprocess.run(["git", "commit", "-m", commit_msg], cwd=repo_root, check=True)
    print(f"Committed: {commit_msg}")


if __name__ == "__main__":
    main()
