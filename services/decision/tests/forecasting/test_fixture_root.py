from datetime import datetime
from zoneinfo import ZoneInfo

from gridos.forecasting.serve import _day_ahead_prices
from gridos.v1 import device_pb2


def test_public_fixture_root_can_be_isolated(tmp_path, monkeypatch) -> None:
    prices = tmp_path / "ercot-prices"
    prices.mkdir()
    (prices / "dam-spp-week.csv").write_text(
        "Delivery Date,Hour Ending,Repeated Hour Flag,Settlement Point,Settlement Point Price,Provenance\n"
        "09/26/2026,13:00,N,LZ_AEN,-15,SIMULATED\n"
    )
    (prices / "PROVENANCE.md").write_text("Provenance: SIMULATED\n")
    monkeypatch.setenv("GRIDOS_PUBLIC_FIXTURES_DIR", str(tmp_path))

    assert _day_ahead_prices() == {
        ("LZ_AEN", datetime(2026, 9, 26, 12, tzinfo=ZoneInfo("America/Chicago"))): (
            -15,
            device_pb2.DATA_PROVENANCE_SIMULATED,
        )
    }
