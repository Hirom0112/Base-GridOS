import csv
from pathlib import Path

import fixtures
import pytest


def test_price_fixtures_keep_lz_aen_prices(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    source_dir = tmp_path / "data/texas/ercot-prices"
    source_dir.mkdir(parents=True)
    with (source_dir / "dam-spp-2025.csv").open("w", newline="") as stream:
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
        writer.writerow(["01/01/2025", "01:00", "N", "LZ_AEN", "21.5"])
        writer.writerow(["01/01/2025", "01:00", "N", "LZ_HOUSTON", "22.0"])
    with (source_dir / "rtm-spp-2025.csv").open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(
            [
                "Delivery Date",
                "Delivery Hour",
                "Delivery Interval",
                "Repeated Hour Flag",
                "Settlement Point Name",
                "Settlement Point Type",
                "Settlement Point Price",
            ]
        )
        writer.writerow(["01/01/2025", "1", "1", "N", "LZ_AEN", "LZ", "19.25"])
        writer.writerow(["01/01/2025", "1", "1", "N", "LZ_HOUSTON", "LZ", "18.0"])
    monkeypatch.setattr(fixtures, "ROOT", tmp_path)
    monkeypatch.setattr(fixtures, "PUBLIC", tmp_path / "public")

    fixtures.price_fixtures()

    prices = tmp_path / "public/ercot-prices"
    with (prices / "dam-spp-week.csv").open(newline="") as stream:
        dam = {
            (row["Settlement Point"], row["Settlement Point Price"])
            for row in csv.DictReader(stream)
        }
    with (prices / "rtm-spp-week.csv").open(newline="") as stream:
        rtm = {
            (row["Settlement Point Name"], row["Settlement Point Price"])
            for row in csv.DictReader(stream)
        }
    assert dam == {("LZ_AEN", "21.5"), ("LZ_HOUSTON", "22.0")}
    assert rtm == {("LZ_AEN", "19.25"), ("LZ_HOUSTON", "18.0")}
