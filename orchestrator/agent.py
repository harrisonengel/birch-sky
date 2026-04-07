import json
import os
import re
import sys
import time

import anthropic

from .models import Task, AgentResult

client = anthropic.Anthropic()


def _parse_response(raw: str) -> dict:
    """Parse JSON from Claude's response, handling markdown fences and preamble."""
    # Strip markdown fences
    clean = re.sub(r"```json\s*", "", raw)
    clean = re.sub(r"```\s*", "", clean)
    clean = clean.strip()

    try:
        return json.loads(clean)
    except json.JSONDecodeError:
        pass

    # Try to extract first JSON object
    match = re.search(r"\{[\s\S]*\}", clean)
    if match:
        try:
            return json.loads(match.group())
        except json.JSONDecodeError:
            pass

    # Fallback: treat entire response as plain output
    return {"output": raw, "proposed_tasks": [], "open_questions": [], "files_written": []}


def run(task: Task, system_prompt: str) -> AgentResult:
    try:
        response = client.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=8096,
            system=system_prompt,
            messages=[{"role": "user", "content": task.goal}],
        )
    except anthropic.RateLimitError:
        print("Rate limited. Waiting 30s and retrying...")
        time.sleep(30)
        try:
            response = client.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=8096,
                system=system_prompt,
                messages=[{"role": "user", "content": task.goal}],
            )
        except anthropic.APIError as e:
            print(f"API error after retry: {e}", file=sys.stderr)
            return AgentResult(output=f"API ERROR: {e}")
    except anthropic.APIError as e:
        print(f"API error: {e}", file=sys.stderr)
        return AgentResult(output=f"API ERROR: {e}")

    raw = response.content[0].text
    data = _parse_response(raw)

    # Write outputs to disk
    out_dir = f"outputs/{task.id}"
    os.makedirs(out_dir, exist_ok=True)

    out_path = f"{out_dir}/output.md"
    with open(out_path, "w") as f:
        f.write(data.get("output", ""))

    meta = {
        "proposed_tasks": data.get("proposed_tasks", []),
        "open_questions": data.get("open_questions", []),
        "files_written": data.get("files_written", []),
    }
    with open(f"{out_dir}/meta.json", "w") as f:
        json.dump(meta, f, indent=2)

    # Append open questions
    if data.get("open_questions"):
        os.makedirs(".agent/context", exist_ok=True)
        with open(".agent/context/open_questions.md", "a") as f:
            f.write(f"\n## From task {task.id}\n")
            for q in data["open_questions"]:
                f.write(f"- {q}\n")

    return AgentResult(
        output=data.get("output", ""),
        proposed_tasks=data.get("proposed_tasks", []),
        open_questions=data.get("open_questions", []),
        files_written=[out_path] + data.get("files_written", []),
    )
