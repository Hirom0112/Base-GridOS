import argparse
import json
from datetime import UTC, datetime
from pathlib import Path

import pyarrow as pa
import pyarrow.parquet as parquet

ROOT = Path(__file__).parents[3]


def timestamp(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(UTC)


def forecast_rows(path: Path, document: dict[str, object]) -> list[dict[str, object]]:
    properties = document["properties"]
    issued_at = timestamp(properties["generatedAt"])
    city = path.stem.removesuffix("_forecast")
    return [
        {
            "record_type": "forecast_period",
            "location": city,
            "issued_at": issued_at,
            "valid_from": timestamp(period["startTime"]),
            "valid_to": timestamp(period["endTime"]),
            "value_kind": "forecast",
            "payload": json.dumps(period, sort_keys=True),
            "provenance": "CONFIRMED_PUBLIC",
        }
        for period in properties["periods"]
    ]


def alert_rows(path: Path, document: dict[str, object]) -> list[dict[str, object]]:
    city = path.stem.removesuffix("_alerts")
    rows = []
    for feature in document["features"]:
        properties = feature["properties"]
        rows.append(
            {
                "record_type": "alert",
                "location": city,
                "issued_at": timestamp(properties["sent"]),
                "valid_from": timestamp(properties["onset"] or properties["effective"]),
                "valid_to": timestamp(properties["ends"] or properties["expires"]),
                "value_kind": "forecast",
                "payload": json.dumps(properties, sort_keys=True),
                "provenance": "CONFIRMED_PUBLIC",
            }
        )
    return rows


def normalize_nws(paths: list[Path], destination: Path | None = None) -> Path:
    rows = []
    for path in paths:
        document = json.loads(path.read_text())
        parser = forecast_rows if path.stem.endswith("_forecast") else alert_rows
        rows.extend(parser(path, document))
    target = destination or ROOT / ".local/normalized/nws.parquet"
    target.parent.mkdir(parents=True, exist_ok=True)
    parquet.write_table(pa.Table.from_pylist(rows), target)
    return target


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("paths", nargs="+", type=Path)
    args = parser.parse_args()
    normalize_nws(args.paths)


if __name__ == "__main__":
    main()
