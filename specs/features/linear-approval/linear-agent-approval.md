# Linear-Based Agent Approval

## Problem

The orchestrator's review gates (SPEC/DECIDE tasks, proposal triage) block on
CLI `input()`, requiring a human at the terminal. This prevents unattended
agent operation and makes it impossible to unblock agents from a phone.

## Solution

Replace blocking `input()` calls with an asynchronous Linear-based approval
flow when `--linear-approval` is passed to the orchestrator.

### Flow

1. **Agent needs approval** (review gate or proposal triage):
   - Posts reasoning/summary as a **comment** on the Linear issue.
   - Moves the issue to **Blocked** (falls back to "In Review" if Blocked
     doesn't exist in the workflow).
   - Assigns the issue to the configured approver
     (`LINEAR_APPROVAL_ASSIGNEE_ID` env var).
   - Sets local task status to `REVIEW`.

2. **Human responds from phone**:
   - Reads the comment on the Linear issue.
   - Adds a reply comment with their decision.
   - Moves the issue back to **Todo**.

3. **Orchestrator picks it up**:
   - `pull_from_linear` detects a REVIEW task whose issue is back in Todo.
   - Fetches comments on the issue; finds the human's response (latest
     comment after the agent's question).
   - If response is an approval keyword (`approved`, `lgtm`, `yes`, `ship it`):
     marks task Done.
   - Otherwise: treats the response as feedback, resumes the Claude session
     via `--resume`, and re-posts for another round of approval.

4. **Proposals**: When the completed task has proposed follow-ups, they are
   listed in the approval comment. The human's response can include which
   proposals to approve (e.g. "approved. follow up on 1 and 3").

### Configuration

| Env var | Required | Description |
|---|---|---|
| `LINEAR_APPROVAL_ASSIGNEE_ID` | Yes (for `--linear-approval`) | Linear user ID to assign approval tickets to |

### CLI

```
python -m orchestrator --identity programmer --linear-approval
```

### Fallback

When `--linear-approval` is **not** set, the existing CLI `input()` review
gates remain unchanged. The two modes are mutually exclusive per run.
