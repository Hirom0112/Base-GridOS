import argparse
import csv
from collections import defaultdict
from datetime import datetime
from pathlib import Path

import pyarrow as pa
import pyarrow.parquet as parquet

ROOT = Path(__file__).parents[3]
EVENT_SCHEMA = pa.schema(
    [
        ("outage_id", pa.int64()),
        ("utility", pa.string()),
        ("county", pa.string()),
        ("started_at", pa.timestamp("us")),
        ("ended_at", pa.timestamp("us")),
        ("customers_tracked", pa.int64()),
        ("customers_out", pa.int64()),
        ("duration_minutes", pa.float64()),
        ("provenance", pa.string()),
    ]
)


def event(row: dict[str, str]) -> dict[str, object]:
    return {
        "outage_id": int(row["outage_id"]),
        "utility": row["utility"],
        "county": row["county"],
        "started_at": datetime.fromisoformat(row["min_updated_time"]),
        "ended_at": datetime.fromisoformat(row["max_updated_time"]),
        "customers_tracked": int(row["customers_tracked"]),
        "customers_out": int(row["customers_out"]),
        "duration_minutes": float(row["outage_duration_mins"]),
        "provenance": "CONFIRMED_PUBLIC",
    }


def normalize_outages(source: Path, output_dir: Path | None = None) -> tuple[Path, Path]:
    destination = output_dir or ROOT / ".local/normalized"
    destination.mkdir(parents=True, exist_ok=True)
    events_path = destination / "outages.parquet"
    rate_parts: dict[tuple[str, str], list[float]] = defaultdict(lambda: [0.0, 0.0])
    with (
        source.open(newline="") as stream,
        parquet.ParquetWriter(events_path, EVENT_SCHEMA) as writer,
    ):
        batch = []
        for raw in csv.DictReader(stream):
            normalized = event(raw)
            batch.append(normalized)
            month = normalized["started_at"].strftime("%Y-%m")
            key = (normalized["county"], month)
            duration = normalized["duration_minutes"]
            rate_parts[key][0] += normalized["customers_out"] * duration
            rate_parts[key][1] += normalized["customers_tracked"] * duration
            if len(batch) == 10_000:
                writer.write_table(pa.Table.from_pylist(batch, schema=EVENT_SCHEMA))
                batch.clear()
        if batch:
            writer.write_table(pa.Table.from_pylist(batch, schema=EVENT_SCHEMA))
    rates = [
        {
            "county": county,
            "month": month,
            "outage_rate": values[0] / values[1],
            "provenance": "DERIVED",
        }
        for (county, month), values in sorted(rate_parts.items())
    ]
    rates_path = destination / "outage_rates.parquet"
    parquet.write_table(pa.Table.from_pylist(rates), rates_path)
    return events_path, rates_path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    normalize_outages(args.source)


if __name__ == "__main__":
    main()
