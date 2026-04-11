from __future__ import annotations

import argparse

from .runner import run


def main() -> None:
    parser = argparse.ArgumentParser(
        description="IE Agent Harness — standalone buyer-agent runner"
    )
    parser.add_argument(
        "-c", "--config", required=True, help="Path to infrastructure YAML config"
    )
    parser.add_argument(
        "-s",
        "--session",
        required=True,
        help="Path to session YAML (starting_context, max_turns)",
    )
    parser.add_argument(
        "-i", "--input", required=True, help="User input / instruction for the agent"
    )
    args = parser.parse_args()
    run(args.config, args.session, args.input)


if __name__ == "__main__":
    main()
