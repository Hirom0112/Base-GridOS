import json
from pathlib import Path

REASONS = {"SCHEMA", "RANGE", "SEQUENCE", "FRESHNESS"}


def write_quarantine(
    directory: Path,
    source_name: str,
    record: dict[str, str],
    reason: str,
) -> None:
    if reason not in REASONS:
        raise ValueError(f"invalid quarantine reason: {reason}")
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / f"{Path(source_name).stem}.jsonl"
    with path.open("a") as stream:
        stream.write(json.dumps({"reason": reason, "record": record}, sort_keys=True) + "\n")
