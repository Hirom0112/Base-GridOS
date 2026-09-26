import csv
import json
from pathlib import Path

from normalize.outages import normalize_outages


def test_range_failure_lands_in_quarantine(tmp_path: Path) -> None:
    source = tmp_path / "outages.csv"
    fields = [
        "outage_id",
        "utility",
        "county",
        "min_updated_time",
        "max_updated_time",
        "customers_tracked",
        "customers_out",
        "outage_duration_mins",
    ]
    with source.open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(fields)
        writer.writerow(["1", "A", "Travis", "2023-01-01", "2023-01-01", "10", "2", "5"])
        writer.writerow(["2", "A", "Travis", "2023-01-01", "2023-01-01", "10", "12", "5"])

    events, _ = normalize_outages(
        source,
        tmp_path / "normalized",
        quarantine_dir=tmp_path / ".local/quarantine",
    )

    import pyarrow.parquet as parquet

    assert parquet.read_table(events).num_rows == 1
    rejected = [
        json.loads(line)
        for line in (tmp_path / ".local/quarantine/outages.jsonl").read_text().splitlines()
    ]
    assert rejected == [
        {
            "reason": "RANGE",
            "record": dict(
                zip(
                    fields,
                    ["2", "A", "Travis", "2023-01-01", "2023-01-01", "10", "12", "5"],
                    strict=True,
                )
            ),
        }
    ]
