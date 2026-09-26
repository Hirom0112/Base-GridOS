import csv
from datetime import datetime, timedelta
from pathlib import Path
from zoneinfo import ZoneInfo

from gridos.forecasting.load import LoadObservation, forecast_load

ROOT = Path(__file__).parents[4]
CENTRAL = ZoneInfo("America/Chicago")


def test_load_baseline_reproduces_held_out_reshiwr_coast_day() -> None:
    source = ROOT / "testdata/fixtures/public/load-profiles/residential-week.csv"
    observations = []
    with source.open(newline="") as stream:
        for row in csv.DictReader(stream):
            if row["PType_WZ"] != "RESHIWR_COAST":
                continue
            start = datetime.fromisoformat(row["Date"]).replace(tzinfo=CENTRAL)
            for index in range(96):
                observations.append(
                    LoadObservation(
                        timestamp=start + timedelta(minutes=15 * index),
                        energy_kwh=float(row[f"int_kWh{index + 1}"]),
                    )
                )

    issued_at = datetime(2025, 1, 7, tzinfo=CENTRAL)
    training = [item for item in observations if item.timestamp < issued_at]
    actual = [item.energy_kwh for item in observations if item.timestamp >= issued_at]
    forecast = forecast_load(training, issued_at=issued_at, horizon=timedelta(days=1))

    assert forecast.issued_at == issued_at
    assert forecast.horizon == timedelta(days=1)
    assert forecast.training_window[1] < issued_at
    assert forecast.feature_version
    assert forecast.model_version
    assert len(forecast.values_kwh) == len(actual) == 96
    assert (
        sum(
            abs(predicted - observed)
            for predicted, observed in zip(forecast.values_kwh, actual, strict=True)
        )
        / 96
        < 0.16
    )
    assert (
        sum(
            lower <= observed <= upper
            for lower, observed, upper in zip(
                forecast.lower_kwh, actual, forecast.upper_kwh, strict=True
            )
        )
        >= 77
    )
