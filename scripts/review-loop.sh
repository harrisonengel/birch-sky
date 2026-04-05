#!/usr/bin/env bash
# review-loop.sh — Address persona review feedback by resuming a Claude Code session.
#
# Usage:
#   ./scripts/review-loop.sh <pr_number> [session_id]
#
# Designed to run inside a git worktree created by scripts/claude-worktree.sh.
# If session_id is omitted, it is read from .claude-session-id at the repo root.
# The session ID is updated after each Claude turn and written back to that file.
#
# The loop:
#   1. Poll check_reviews.py until all verdicts are in.
#   2. If all approved: squash-merge the PR, remove the worktree, delete the branch.
#   3. If changes requested, collect the review text and pass it to Claude via
#      `claude -p --resume <session_id>`, which continues the original session.
#   4. Claude edits files, commits, and pushes. Then loop back to step 1.
#
# Requirements:
#   - claude CLI in PATH (Claude Code, https://claude.ai/code)
#   - gh CLI in PATH and authenticated
#   - GH_TOKEN or GITHUB_TOKEN set (for GitHub API calls in Python scripts)
#   - Python 3

set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
REPO="harrisonengel/birch-sky"
MAIN_REPO="$HOME/src/birch-sky"
MAX_ITERATIONS=5
POLL_INTERVAL=30   # seconds between verdict polls

# ── Paths ─────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"
SCRIPTS_DIR="$REPO_ROOT/.github/scripts"
SESSION_FILE="$REPO_ROOT/.claude-session-id"
BRANCH="$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)"

# ── Args ──────────────────────────────────────────────────────────────────────
if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <pr_number> [session_id]" >&2
  exit 1
fi

PR_NUMBER="$1"

if [[ $# -ge 2 ]]; then
  SESSION_ID="$2"
  echo "$SESSION_ID" > "$SESSION_FILE"
elif [[ -f "$SESSION_FILE" ]]; then
  SESSION_ID="$(cat "$SESSION_FILE")"
else
  echo "Error: no session ID provided and $SESSION_FILE not found." >&2
  echo "Pass the session ID as the second argument, or store it in $SESSION_FILE." >&2
  echo "To start a fresh session: claude -p \"<prompt>\" --output-format json | tee /tmp/r.json; jq -r .session_id /tmp/r.json > .claude-session-id" >&2
  exit 1
fi

echo "PR #$PR_NUMBER  session $SESSION_ID"

# ── Temp files (cleaned up on exit) ───────────────────────────────────────────
VERDICT_FILE="$(mktemp /tmp/verdict.XXXXXX.json)"
FEEDBACK_FILE="$(mktemp /tmp/feedback.XXXXXX.md)"
ITER_FILE="$(mktemp /tmp/iter.XXXXXX.txt)"
RESPONSE_FILE="$(mktemp /tmp/claude_response.XXXXXX.json)"
trap 'rm -f "$VERDICT_FILE" "$FEEDBACK_FILE" "$ITER_FILE" "$RESPONSE_FILE"' EXIT

# ── Loop ──────────────────────────────────────────────────────────────────────
while true; do
  HEAD_SHA="$(git -C "$REPO_ROOT" rev-parse HEAD)"
  echo ""
  echo "[$(date '+%H:%M:%S')] Checking verdicts  PR #$PR_NUMBER  HEAD $HEAD_SHA"

  python3 "$SCRIPTS_DIR/check_reviews.py" \
    --repo "$REPO" \
    --pr  "$PR_NUMBER" \
    --head-sha "$HEAD_SHA" \
    --output "$VERDICT_FILE"

  all_approved="$(python3 -c "
import json
d = json.load(open('$VERDICT_FILE'))
print('true' if d['all_approved'] else 'false')
")"

  pending_count="$(python3 -c "
import json
d = json.load(open('$VERDICT_FILE'))
print(len(d.get('pending', [])))
")"

  if [[ "$all_approved" == "true" ]]; then
    echo "All personas approved. Merging PR #$PR_NUMBER..."
    gh pr merge "$PR_NUMBER" --repo "$REPO" --squash --delete-branch

    echo "Cleaning up worktree for branch '$BRANCH'..."
    cd "$MAIN_REPO"
    git fetch origin
    git worktree remove "$REPO_ROOT" --force
    git branch -d "$BRANCH" 2>/dev/null || true
    echo "Done. Branch '$BRANCH' landed and worktree removed."
    exit 0
  fi

  if [[ "$pending_count" -gt 0 ]]; then
    echo "$pending_count review(s) still pending. Waiting ${POLL_INTERVAL}s..."
    sleep "$POLL_INTERVAL"
    continue
  fi

  # ── All reviews in, changes requested ────────────────────────────────────
  python3 "$SCRIPTS_DIR/count_iterations.py" \
    --repo "$REPO" \
    --pr   "$PR_NUMBER" \
    --output "$ITER_FILE"
  iteration="$(cat "$ITER_FILE")"

  if [[ "$iteration" -ge "$MAX_ITERATIONS" ]]; then
    echo "Reached max fix iterations ($MAX_ITERATIONS). Stopping." >&2
    exit 1
  fi

  changes_personas="$(python3 -c "
import json
d = json.load(open('$VERDICT_FILE'))
print(', '.join(p for p, v in d['verdicts'].items() if v == 'CHANGES REQUESTED'))
")"
  echo "Changes requested by: $changes_personas  (iteration $((iteration + 1))/$MAX_ITERATIONS)"

  # Collect the actual review text
  python3 "$SCRIPTS_DIR/collect_feedback.py" \
    --repo         "$REPO" \
    --pr           "$PR_NUMBER" \
    --head-sha     "$HEAD_SHA" \
    --verdict-file "$VERDICT_FILE" \
    --output       "$FEEDBACK_FILE"

  feedback="$(cat "$FEEDBACK_FILE")"
  if [[ -z "$feedback" ]]; then
    echo "Could not collect review feedback yet. Retrying in ${POLL_INTERVAL}s..."
    sleep "$POLL_INTERVAL"
    continue
  fi

  # ── Resume the Claude session ─────────────────────────────────────────────
  echo "Resuming session $SESSION_ID..."

  claude -p "The persona reviews have come back with feedback. Please address each issue below, then commit and push your changes.

$feedback" \
    --resume "$SESSION_ID" \
    --allowedTools "Read,Edit,Bash(git add *),Bash(git commit *),Bash(git push *),Bash(git diff *),Bash(git status *)" \
    --output-format json \
    > "$RESPONSE_FILE"

  # Persist the updated session ID so the next turn continues the same thread
  new_session="$(python3 -c "
import json, sys
try:
    r = json.load(open('$RESPONSE_FILE'))
    print(r.get('session_id', ''))
except Exception:
    print('')
" 2>/dev/null)"

  if [[ -n "$new_session" ]]; then
    SESSION_ID="$new_session"
    echo "$SESSION_ID" > "$SESSION_FILE"
    echo "Session updated: $SESSION_ID"
  fi

  echo "Claude finished. Waiting ${POLL_INTERVAL}s for new reviews..."
  sleep "$POLL_INTERVAL"
done
