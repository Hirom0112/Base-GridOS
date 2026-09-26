import csv
from pathlib import Path

from normalize.ercot_prices import normalize_dam


def test_dam_prices_preserve_fall_back_hours(tmp_path: Path) -> None:
    source = tmp_path / "dam.csv"
    with source.open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(
            [
                "Delivery Date",
                "Hour Ending",
                "Repeated Hour Flag",
                "Settlement Point",
                "Settlement Point Price",
            ]
        )
        writer.writerows(
            [
                ["11/02/2025", "02:00", "N", "LZ_HOUSTON", "45.35"],
                ["11/02/2025", "02:00", "Y", "LZ_HOUSTON", "46.86"],
            ]
        )
    destination = tmp_path / "dam.parquet"

    normalize_dam(source, destination)

    import pyarrow.parquet as parquet

    rows = parquet.read_table(destination).to_pylist()
    assert {row["provenance"] for row in rows} == {"CONFIRMED_PUBLIC"}
    assert {row["unit"] for row in rows} == {"USD_per_MWh"}
    assert len({row["timestamp_utc"] for row in rows}) == 2
    assert all(row["timestamp_local"].tzinfo is not None for row in rows)
    assert {row["repeated_hour"] for row in rows} == {False, True}
