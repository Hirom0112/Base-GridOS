from pathlib import Path

from normalize.nws import normalize_nws

ROOT = Path(__file__).parents[2]


def test_nws_snapshots_become_forecast_records(tmp_path: Path) -> None:
    destination = tmp_path / "nws.parquet"
    normalize_nws(
        [
            ROOT / "data/texas/weather/austin_forecast.json",
            ROOT / "data/texas/weather/houston_alerts.json",
        ],
        destination,
    )

    import pyarrow.parquet as parquet

    rows = parquet.read_table(destination).to_pylist()
    assert {row["record_type"] for row in rows} == {"forecast_period", "alert"}
    assert {row["value_kind"] for row in rows} == {"forecast"}
    assert all(row["issued_at"] for row in rows)
    assert all(row["valid_from"] for row in rows)
    assert all(row["valid_to"] for row in rows)
