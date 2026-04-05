#!/usr/bin/env python3
"""Generate a PR review from a specific persona using the Anthropic API."""

import argparse
import json
import os
import sys
import urllib.request

MAX_DIFF_CHARS = 80000


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--persona", required=True)
    parser.add_argument("--skill-file", required=True)
    parser.add_argument("--diff-file", required=True)
    parser.add_argument("--pr-title", required=True)
    parser.add_argument("--pr-body-file", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        print("Error: ANTHROPIC_API_KEY environment variable is required", file=sys.stderr)
        sys.exit(1)

    with open(args.skill_file) as f:
        skill_content = f.read()
    with open(args.diff_file) as f:
        diff_content = f.read()
    with open(args.pr_body_file) as f:
        pr_body = f.read()

    if len(diff_content) > MAX_DIFF_CHARS:
        diff_content = diff_content[:MAX_DIFF_CHARS] + "\n\n[... diff truncated ...]"

    # Use the SKILL.md content as system prompt, replacing the $ARGUMENTS placeholder
    system_prompt = skill_content.replace("$ARGUMENTS", "").rstrip()

    user_message = f"""Review this pull request from your persona's perspective.

## PR Title
{args.pr_title}

## PR Description
{pr_body}

## Diff
```diff
{diff_content}
```

Provide a focused review from your specific perspective. Be concrete and actionable.
Identify issues, risks, or improvements relevant to your role.
If the changes look good from your perspective, say so briefly and explain why.
Keep your review concise (under 500 words).
"""

    request_body = json.dumps({
        "model": "claude-sonnet-4-20250514",
        "max_tokens": 1024,
        "system": system_prompt,
        "messages": [{"role": "user", "content": user_message}],
    }).encode()

    req = urllib.request.Request(
        "https://api.anthropic.com/v1/messages",
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

    review_text = result["content"][0]["text"]

    persona_display = args.persona.replace("-", " ").title()
    formatted = f"## Persona Review: {persona_display}\n\n{review_text}"

    with open(args.output, "w") as f:
        f.write(formatted)

    print(f"Review generated for persona: {persona_display}")


if __name__ == "__main__":
    main()
