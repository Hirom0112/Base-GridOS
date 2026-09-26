import csv
from pathlib import Path

from normalize.outages import normalize_outages


def test_outages_produce_one_rate_per_county_month(tmp_path: Path) -> None:
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
    rows = [
        ["1", "Utility A", "Travis", "2023-01-02", "2023-01-02", "100", "10", "60"],
        ["2", "Utility A", "Travis", "2023-01-03", "2023-01-03", "100", "20", "30"],
        ["3", "Utility B", "Harris", "2023-02-01", "2023-02-01", "200", "50", "15"],
    ]
    with source.open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(fields)
        writer.writerows(rows)

    events, rates = normalize_outages(source, tmp_path)

    import pyarrow.parquet as parquet

    event_rows = parquet.read_table(events).to_pylist()
    rate_rows = parquet.read_table(rates).to_pylist()
    assert event_rows[0]["duration_minutes"] == 60.0
    assert event_rows[0]["customers_out"] == 10
    assert {(row["county"], row["month"]) for row in rate_rows} == {
        ("Travis", "2023-01"),
        ("Harris", "2023-02"),
    }
    assert (
        next(row for row in rate_rows if row["county"] == "Travis")["outage_rate"]
        == 0.13333333333333333
    )
