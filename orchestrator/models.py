from dataclasses import dataclass, field
from enum import Enum
from typing import Optional
import uuid
import datetime


class TaskType(str, Enum):
    SPEC = "spec"
    IMPLEMENT = "implement"
    REVIEW = "review"
    DECIDE = "decide"
    TEST = "test"


class Status(str, Enum):
    PENDING = "pending"
    READY = "ready"
    IN_PROGRESS = "in_progress"
    REVIEW = "review"
    DONE = "done"
    BLOCKED = "blocked"


@dataclass
class Task:
    goal: str
    type: TaskType
    id: str = field(default_factory=lambda: str(uuid.uuid4())[:8])
    context_refs: list[str] = field(default_factory=list)
    acceptance_criteria: str = ""
    dependencies: list[str] = field(default_factory=list)
    status: Status = Status.PENDING
    proposed_by: str = "human"
    authorized_by: Optional[str] = None
    output_path: Optional[str] = None
    created_at: str = field(default_factory=lambda: datetime.datetime.now().isoformat())
    notes: str = ""
    linear_issue_id: Optional[str] = None
    linear_url: Optional[str] = None


@dataclass
class AgentResult:
    output: str
    proposed_tasks: list[dict] = field(default_factory=list)
    open_questions: list[str] = field(default_factory=list)
    files_written: list[str] = field(default_factory=list)
