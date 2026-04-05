#!/usr/bin/env python3
"""Design-adherence review: checks that src/ code changes adhere to design.md files."""

import argparse
import json
import os
import re
import sys
import urllib.request
from pathlib import Path

MAX_DIFF_CHARS = 60000
API_URL = "https://api.anthropic.com/v1/messages"
MODEL = "claude-sonnet-4-6"


def call_claude(api_key, system_prompt, user_message, max_tokens=1500):
    request_body = json.dumps({
        "model": MODEL,
        "max_tokens": max_tokens,
        "system": system_prompt,
        "messages": [{"role": "user", "content": user_message}],
    }).encode()

    req = urllib.request.Request(
        API_URL,
        data=request_body,
        headers={
            "Content-Type": "application/json",
            "x-api-key": api_key,
            "anthropic-version": "2023-06-01",
        },
    )

    try:
        with urllib.request.urlopen(req) as resp:
            result = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        print(f"Anthropic API error ({e.code}): {body}", file=sys.stderr)
        sys.exit(1)

    return result["content"][0]["text"]


def get_changed_src_folders(diff_content):
    """Return unique folder paths (relative to repo root) that changed under src/."""
    folders = set()
    for line in diff_content.split("\n"):
        if not line.startswith("diff --git a/src/"):
            continue
        match = re.match(r"diff --git a/(src/[^\s]+)", line)
        if match:
            filepath = match.group(1)
            folders.add(str(Path(filepath).parent))
    return sorted(folders)


def collect_design_hierarchy(repo_root, folder):
    """
    Return list of (relative_path, content_or_None) for design.md files
    from src/ down to the changed folder (inclusive), top-first.
    """
    src_root = Path(repo_root) / "src"
    current = Path(repo_root) / folder

    # Build path list from current up to src/ (inclusive)
    path_chain = []
    p = current
    while True:
        path_chain.append(p)
        if p == src_root:
            break
        parent = p.parent
        if parent == p:
            break
        p = parent

    # Reverse: top (src/) → bottom (changed folder)
    designs = []
    for path in reversed(path_chain):
        design_file = path / "design.md"
        rel = str(design_file.relative_to(repo_root))
        if design_file.exists():
            designs.append((rel, design_file.read_text()))
        else:
            designs.append((rel, None))

    return designs


def extract_folder_diff(diff_content, folder):
    """Extract diff chunks for files directly inside `folder`."""
    lines = diff_content.split("\n")
    result = []
    capturing = False

    for line in lines:
        if line.startswith("diff --git"):
            match = re.match(r"diff --git a/([^\s]+)", line)
            if match:
                filepath = match.group(1)
                capturing = str(Path(filepath).parent) == folder
        if capturing:
            result.append(line)

    return "\n".join(result)


def review_folder(api_key, folder, designs, folder_diff, project_context):
    """Call Claude to review one folder. Returns (review_text, has_violations)."""
    present = [(p, c) for p, c in designs if c is not None]
    missing = [p for p, c in designs if c is None]

    design_context = (
        "\n\n".join(f"### `{p}`\n\n{c}" for p, c in present)
        if present
        else "_No design.md files found in this hierarchy._"
    )

    missing_section = ""
    if missing:
        missing_section = (
            "\n## Missing design.md files\n"
            + "\n".join(f"- `{p}` (does not exist)" for p in missing)
            + "\n"
        )

    if len(folder_diff) > MAX_DIFF_CHARS:
        folder_diff = folder_diff[:MAX_DIFF_CHARS] + "\n\n[... diff truncated ...]"

    system_prompt = (
        "You are a strict design-adherence reviewer. "
        "Your job is to evaluate whether code changes follow the project's design.md documents "
        "and whether any important design decisions made during coding are missing from design.md.\n\n"
        "Be concise and direct. Focus only on genuine design violations or undocumented architectural "
        "decisions — not style, naming, or minor implementation details."
    )

    user_message = f"""Review code changes in `{folder}` for design adherence.

## Project Context
{project_context or "(none provided)"}

## Design Document Hierarchy (src/ → changed folder)
{design_context}
{missing_section}
## Code Changes in `{folder}`
```diff
{folder_diff}
```

Evaluate two things:

**1. Design Adherence** — Does the code contradict or ignore the design.md files?
Examples of violations: implementing a different architecture than specified, ignoring documented constraints, adding undocumented external dependencies, breaking described APIs.

**2. Design Coverage** — Did the code introduce patterns or decisions NOT covered by design.md?
Examples: new architectural patterns, new integration points, significant scope changes, major tradeoffs.
Note: this is informational only. The author should update design.md but it does not block the PR.

Respond using EXACTLY this format (no extra sections):

### `{folder}` Design Review

**Adherence**: PASS | VIOLATIONS FOUND
**Coverage**: COMPLETE | NEEDS UPDATE

#### Violations
(bullet list of adherence violations, or "None")

#### Design.md Updates Needed
(bullet list of decisions/patterns the author should document in design.md, or "None")

#### Verdict
**APPROVED** or **CHANGES REQUESTED** — one sentence reason.
"""

    text = call_claude(api_key, system_prompt, user_message)
    has_violations = "CHANGES REQUESTED" in text
    return text, has_violations


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--diff-file", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--project-context-file", default=None)
    args = parser.parse_args()

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        print("Error: ANTHROPIC_API_KEY not set", file=sys.stderr)
        sys.exit(1)

    diff_content = Path(args.diff_file).read_text()

    project_context = ""
    if args.project_context_file and Path(args.project_context_file).exists():
        project_context = Path(args.project_context_file).read_text()

    changed_folders = get_changed_src_folders(diff_content)

    if not changed_folders:
        Path(args.output).write_text(
            "## Design Adherence Review\n\nNo changes to `src/` detected — skipping.\n"
        )
        print("No src/ changes found.")
        return

    print(f"Folders to review: {changed_folders}")

    folder_reviews = []
    any_violations = False

    for folder in changed_folders:
        folder_diff = extract_folder_diff(diff_content, folder)
        if not folder_diff.strip():
            continue

        designs = collect_design_hierarchy(args.repo_root, folder)
        review_text, has_violations = review_folder(
            api_key, folder, designs, folder_diff, project_context
        )
        folder_reviews.append(review_text)
        if has_violations:
            any_violations = True
        print(f"  {'FAIL' if has_violations else 'PASS'}: {folder}")

    if not folder_reviews:
        Path(args.output).write_text(
            "## Design Adherence Review\n\nNo reviewable changes found in `src/`.\n"
        )
        return

    status_line = (
        "> **Status: CHANGES REQUESTED** — One or more folders have design violations. "
        "Resolve the issues below before merging.\n"
        if any_violations
        else "> **Status: APPROVED** — All changed folders adhere to their design documents.\n"
    )

    output = (
        "## Design Adherence Review\n\n"
        + status_line
        + "\n\n---\n\n"
        + "\n\n---\n\n".join(folder_reviews)
    )

    Path(args.output).write_text(output)
    print(f"Review complete. Violations: {any_violations}")

    if any_violations:
        sys.exit(1)


if __name__ == "__main__":
    main()
