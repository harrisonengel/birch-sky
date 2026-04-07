from __future__ import annotations

import json
import os
from dataclasses import asdict
from typing import Optional

from .models import Task, Status, TaskType

QUEUE_PATH = "tasks/backlog.json"


def load() -> list[Task]:
    if not os.path.exists(QUEUE_PATH):
        return []
    with open(QUEUE_PATH) as f:
        data = json.load(f)
    tasks = []
    for t in data:
        t["type"] = TaskType(t["type"])
        t["status"] = Status(t["status"])
        tasks.append(Task(**t))
    return tasks


def save(tasks: list[Task]):
    os.makedirs(os.path.dirname(QUEUE_PATH), exist_ok=True)
    with open(QUEUE_PATH, "w") as f:
        json.dump([asdict(t) for t in tasks], f, indent=2)


def add(task: Task):
    tasks = load()
    tasks.append(task)
    save(tasks)


def get_next_ready() -> Optional[Task]:
    tasks = load()
    done_ids = {t.id for t in tasks if t.status == Status.DONE}
    for t in tasks:
        if t.status == Status.READY:
            if all(dep in done_ids for dep in t.dependencies):
                return t
    return None


def update(task_id: str, **kwargs):
    tasks = load()
    for t in tasks:
        if t.id == task_id:
            for k, v in kwargs.items():
                if k == "type" and isinstance(v, str):
                    v = TaskType(v)
                elif k == "status" and isinstance(v, str):
                    v = Status(v)
                setattr(t, k, v)
    save(tasks)


def pending_proposals() -> list[Task]:
    return [t for t in load() if t.status == Status.PENDING]
