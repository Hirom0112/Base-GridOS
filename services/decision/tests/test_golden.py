import json
from pathlib import Path

from gridos.golden.fixtures import render_fixtures


def test_golden_plans_match_regenerated_fixtures() -> None:
    fixture_directory = Path(__file__).parents[3] / "testdata" / "golden" / "plans"
    regenerated = render_fixtures()
    checked_in = {
        path.stem: json.loads(path.read_text()) for path in sorted(fixture_directory.glob("*.json"))
    }

    assert set(checked_in) == {
        "duplicate-device",
        "expired-window",
        "feasible",
        "infeasible-shortfall",
        "reserve-tight",
        "stale-device-excluded",
        "weather-override",
        "zero-percent-hardware-floor",
    }
    assert checked_in == regenerated
