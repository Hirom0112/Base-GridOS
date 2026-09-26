import argparse
from pathlib import Path

from tools.generation.scenario.model import Scenario


def main() -> None:
    parser = argparse.ArgumentParser(prog="python -m tools.generation.scenario")
    subcommands = parser.add_subparsers(dest="command", required=True)
    validate = subcommands.add_parser("validate")
    validate.add_argument("path", type=Path)
    arguments = parser.parse_args()
    if arguments.command == "validate":
        Scenario.from_yaml(arguments.path)
        print("OK")


if __name__ == "__main__":
    main()
