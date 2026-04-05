#!/usr/bin/env python3
"""Count fix-loop commits on the PR branch to enforce the iteration cap."""

import argparse
import json
import os
import sys
import urllib.request
import urllib.error

FIX_COMMIT_PREFIX = "fix: apply persona review feedback"


def github_get(url, token):
    req = urllib.request.Request(
        url,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, help="owner/repo")
    parser.add_argument("--pr", required=True, help="PR number")
    parser.add_argument("--output", required=True, help="File to write count to")
    args = parser.parse_args()

    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if not token:
        print("Error: GH_TOKEN or GITHUB_TOKEN required", file=sys.stderr)
        sys.exit(1)

    base_url = f"https://api.github.com/repos/{args.repo}"

    # Get base and head SHAs from the PR
    pr_data = github_get(f"{base_url}/pulls/{args.pr}", token)
    base_sha = pr_data["base"]["sha"]
    head_sha = pr_data["head"]["sha"]

    # Get all commits in the PR range (paginated)
    count = 0
    page = 1
    while True:
        url = f"{base_url}/compare/{base_sha}...{head_sha}?per_page=100&page={page}"
        try:
            data = github_get(url, token)
        except urllib.error.HTTPError as e:
            print(f"GitHub API error: {e.code} {e.read().decode()}", file=sys.stderr)
            sys.exit(1)

        commits = data.get("commits", [])
        if not commits:
            break

        for commit in commits:
            author = commit.get("commit", {}).get("author", {}).get("name", "")
            message = commit.get("commit", {}).get("message", "")
            if author == "github-actions[bot]" and message.startswith(FIX_COMMIT_PREFIX):
                count += 1

        # compare endpoint doesn't paginate the same way; if we got less than 100 we're done
        if len(commits) < 100:
            break
        page += 1

    with open(args.output, "w") as f:
        f.write(str(count))

    print(f"Fix iterations so far: {count}")


if __name__ == "__main__":
    main()
