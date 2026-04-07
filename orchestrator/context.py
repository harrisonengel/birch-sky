import os
from .models import Task

ALWAYS_INCLUDE = ["CLAUDE.md", ".agent/context/open_questions.md"]

SYSTEM_PROMPT_TEMPLATE = """You are a senior software engineer working on a specific task.
Always respond in this JSON format:
{{
  "output": "your main response or code here",
  "files_written": ["path/to/file1"],
  "proposed_tasks": [
    {{"goal": "...", "type": "implement|spec|review|test|decide", "acceptance_criteria": "..."}}
  ],
  "open_questions": ["question 1", "question 2"]
}}

{context}
"""


def _read_safe(path: str) -> str:
    if os.path.exists(path):
        with open(path) as f:
            return f"### {path}\n{f.read()}\n"
    return ""


def build(task: Task) -> str:
    parts = []

    for path in ALWAYS_INCLUDE:
        parts.append(_read_safe(path))

    for ref in task.context_refs:
        parts.append(_read_safe(ref))

    # Pull in outputs from dependencies
    from . import taskqueue
    all_tasks = {t.id: t for t in taskqueue.load()}
    for dep_id in task.dependencies:
        dep = all_tasks.get(dep_id)
        if dep and dep.output_path:
            parts.append(_read_safe(dep.output_path))

    context = "\n".join(filter(None, parts))
    return SYSTEM_PROMPT_TEMPLATE.format(context=context)
