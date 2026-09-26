import argparse
import csv
from datetime import UTC, datetime, timedelta
from pathlib import Path
from zoneinfo import ZoneInfo

import pyarrow as pa
import pyarrow.parquet as parquet

CENTRAL = ZoneInfo("America/Chicago")
ROOT = Path(__file__).parents[3]
SCHEMA = pa.schema(
    [
        ("timestamp_local", pa.timestamp("us", tz="America/Chicago")),
        ("timestamp_utc", pa.timestamp("us", tz="UTC")),
        ("profile_type", pa.string()),
        ("weather_zone", pa.string()),
        ("energy", pa.float64()),
        ("unit", pa.string()),
        ("provenance", pa.string()),
    ]
)


def profile_rows(row: dict[str, str]) -> list[dict[str, object]]:
    values = [row[f"int_kWh{index}"] for index in range(1, 101)]
    intervals = [value for value in values if value != ""]
    if len(intervals) not in {96, 100}:
        raise ValueError(f"invalid interval count: {len(intervals)}")
    profile_type, weather_zone = row["PType_WZ"].rsplit("_", 1)
    local_start = datetime.strptime(row["Date"], "%Y-%m-%d %H:%M:%S").replace(tzinfo=CENTRAL)
    utc_start = local_start.astimezone(UTC)
    result = []
    for index, value in enumerate(intervals):
        timestamp_utc = utc_start + timedelta(minutes=index * 15)
        result.append(
            {
                "timestamp_local": timestamp_utc.astimezone(CENTRAL),
                "timestamp_utc": timestamp_utc,
                "profile_type": profile_type,
                "weather_zone": weather_zone,
                "energy": float(value),
                "unit": "kWh",
                "provenance": "CONFIRMED_PUBLIC",
            }
        )
    return result


def normalize_load_profiles(source: Path, destination: Path | None = None) -> Path:
    target = destination or ROOT / ".local/normalized" / f"{source.stem}.parquet"
    target.parent.mkdir(parents=True, exist_ok=True)
    with source.open(newline="") as stream, parquet.ParquetWriter(target, SCHEMA) as writer:
        for row in csv.DictReader(stream):
            writer.write_table(pa.Table.from_pylist(profile_rows(row), schema=SCHEMA))
    return target


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    normalize_load_profiles(args.source)


if __name__ == "__main__":
    main()
