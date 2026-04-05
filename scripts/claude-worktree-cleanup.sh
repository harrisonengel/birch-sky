#!/usr/bin/env bash
# Remove a git worktree and its branch after work is complete.
#
# Usage: scripts/claude-worktree-cleanup.sh <worktree-name>

set -euo pipefail

MAIN_REPO="$HOME/src/birch-sky"

name="${1:-}"
if [[ -z "$name" ]]; then
    echo "Usage: $0 <worktree-name>" >&2
    exit 1
fi

worktree_path="$HOME/src/$name"
cd "$MAIN_REPO"
git worktree remove "$worktree_path"
git branch -d "$name"
