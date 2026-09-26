import csv
from pathlib import Path

import pytest
from normalize.ercot_load_profiles import normalize_load_profiles


def write_profile(path: Path, count: int) -> None:
    fields = ["PType_WZ", "Date", *[f"int_kWh{i}" for i in range(1, 101)], "ADDTIME"]
    values = ["RESHIPV_COAST", "2025-11-02 00:00:00"]
    values.extend("0.25" if index <= count else "" for index in range(1, 101))
    values.append("2025-11-03 00:00:00")
    with path.open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(fields)
        writer.writerow(values)


def test_load_profiles_become_dst_aware_long_rows(tmp_path: Path) -> None:
    source = tmp_path / "profiles.csv"
    destination = tmp_path / "profiles.parquet"
    write_profile(source, 100)

    normalize_load_profiles(source, destination)

    import pyarrow.parquet as parquet

    rows = parquet.read_table(destination).to_pylist()
    assert len(rows) == 100
    assert {row["profile_type"] for row in rows} == {"RESHIPV"}
    assert {row["weather_zone"] for row in rows} == {"COAST"}
    assert {row["unit"] for row in rows} == {"kWh"}
    assert len({row["timestamp_utc"] for row in rows}) == 100


def test_load_profiles_reject_wrong_interval_count(tmp_path: Path) -> None:
    source = tmp_path / "profiles.csv"
    write_profile(source, 95)

    with pytest.raises(ValueError, match="interval count"):
        normalize_load_profiles(source, tmp_path / "profiles.parquet")
