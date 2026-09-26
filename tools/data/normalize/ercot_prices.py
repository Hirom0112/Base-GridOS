import argparse
import csv
from datetime import UTC, datetime, timedelta
from pathlib import Path
from zoneinfo import ZoneInfo

import pyarrow as pa
import pyarrow.parquet as parquet

CENTRAL = ZoneInfo("America/Chicago")
ROOT = Path(__file__).parents[3]


def local_timestamp(date: str, hour: int, minutes: int, repeated: str) -> datetime:
    day = datetime.strptime(date, "%m/%d/%Y")
    return (day + timedelta(hours=hour, minutes=minutes)).replace(
        tzinfo=CENTRAL, fold=repeated == "Y"
    )


def normalized_row(
    timestamp: datetime,
    settlement_point: str,
    price: str,
    repeated: str,
) -> dict[str, object]:
    return {
        "timestamp_local": timestamp,
        "timestamp_utc": timestamp.astimezone(UTC),
        "settlement_point": settlement_point,
        "price": float(price),
        "unit": "USD_per_MWh",
        "provenance": "CONFIRMED_PUBLIC",
        "repeated_hour": repeated == "Y",
    }


def write_rows(rows: list[dict[str, object]], destination: Path) -> Path:
    destination.parent.mkdir(parents=True, exist_ok=True)
    parquet.write_table(pa.Table.from_pylist(rows), destination)
    return destination


def normalize_dam(source: Path, destination: Path | None = None) -> Path:
    rows = []
    with source.open(newline="") as stream:
        for row in csv.DictReader(stream):
            hour_ending = int(row["Hour Ending"].split(":")[0])
            timestamp = local_timestamp(
                row["Delivery Date"],
                hour_ending - 1,
                0,
                row["Repeated Hour Flag"],
            )
            rows.append(
                normalized_row(
                    timestamp,
                    row["Settlement Point"],
                    row["Settlement Point Price"],
                    row["Repeated Hour Flag"],
                )
            )
    target = destination or ROOT / ".local/normalized" / f"{source.stem}.parquet"
    return write_rows(rows, target)


def normalize_rtm(source: Path, destination: Path | None = None) -> Path:
    rows = []
    with source.open(newline="") as stream:
        for row in csv.DictReader(stream):
            timestamp = local_timestamp(
                row["Delivery Date"],
                int(row["Delivery Hour"]) - 1,
                (int(row["Delivery Interval"]) - 1) * 15,
                row["Repeated Hour Flag"],
            )
            normalized = normalized_row(
                timestamp,
                row["Settlement Point Name"],
                row["Settlement Point Price"],
                row["Repeated Hour Flag"],
            )
            normalized["settlement_point_type"] = row["Settlement Point Type"]
            rows.append(normalized)
    target = destination or ROOT / ".local/normalized" / f"{source.stem}.parquet"
    return write_rows(rows, target)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("market", choices=("dam", "rtm"))
    parser.add_argument("source", type=Path)
    args = parser.parse_args()
    if args.market == "dam":
        normalize_dam(args.source)
        return
    normalize_rtm(args.source)


if __name__ == "__main__":
    main()
