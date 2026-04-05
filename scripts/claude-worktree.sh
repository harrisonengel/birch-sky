#!/usr/bin/env bash
# Create a git worktree for a new coding task and open Claude Code in it.
#
# Usage: scripts/claude-worktree.sh <worktree-name>
#
# Creates ~/src/<name> as a worktree on a new branch called <name>,
# then launches Claude Code inside it.

set -euo pipefail

MAIN_REPO="$HOME/src/birch-sky"

name="${1:-}"
if [[ -z "$name" ]]; then
    echo "Usage: $0 <worktree-name>" >&2
    exit 1
fi

worktree_path="$HOME/src/$name"
git -C "$MAIN_REPO" worktree add "$worktree_path" -b "$name"
cd "$worktree_path"
claude
