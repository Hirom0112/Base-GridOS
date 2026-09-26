import json
from collections import Counter
from pathlib import Path

import h3

from tools.generation.fleet.generate import generate_austin_fleet

AUSTIN_BOUNDS = (29.95, 30.90, -98.30, -97.00)
ADDRESS_FIELDS = {"address", "city", "county", "latitude", "longitude", "postal_code", "street"}


def records() -> list[dict[str, object]]:
    return [
        json.loads(line) for line in generate_austin_fleet(seed=20260926, size=5000).splitlines()
    ]


def test_austin_fleet_is_deterministic() -> None:
    first = generate_austin_fleet(seed=20260926, size=5000)
    second = generate_austin_fleet(seed=20260926, size=5000)

    assert first == second


def test_austin_cells_stay_in_bounds() -> None:
    latitude_min, latitude_max, longitude_min, longitude_max = AUSTIN_BOUNDS
    cells = {str(record["h3_cell"]) for record in records()}

    assert 200 <= len(cells) <= 600
    for cell in cells:
        latitude, longitude = h3.cell_to_latlng(cell)
        assert latitude_min <= latitude <= latitude_max
        assert longitude_min <= longitude <= longitude_max


def test_austin_urban_density_thins_outward() -> None:
    downtown = h3.latlng_to_cell(30.27, -97.74, 7)
    counts = Counter(str(record["h3_cell"]) for record in records())
    bands: list[list[int]] = [[], [], []]
    for cell, site_count in counts.items():
        distance = h3.grid_distance(downtown, cell)
        bands[0 if distance <= 5 else 1 if distance <= 12 else 2].append(site_count)
        latitude, longitude = h3.cell_to_latlng(cell)
        assert 30.02 < latitude < 30.83
        assert -98.22 < longitude < -97.08
    assert all(bands)
    mean = [sum(band) / len(band) for band in bands]
    assert mean[0] > mean[1] > mean[2]


def test_austin_recorded_sites_match_fleet() -> None:
    fixture_path = Path(__file__).parents[3] / "testdata/fixtures/api/FleetService/ListSites.json"
    fixture = json.loads(fixture_path.read_text())
    recorded = {
        site["aggregate"]["h3Cell"]: int(site["aggregate"]["siteCount"])
        for site in fixture["sites"]
    }
    expected = Counter(str(record["h3_cell"]) for record in records())
    assert recorded == expected


def test_austin_capacity_matches_devices() -> None:
    fleet = records()
    capacity_kw = sum(float(device["max_discharge_kw"]) for device in fleet)

    assert capacity_kw == 42_629.727
    assert capacity_kw >= 20_000


def test_austin_fleet_has_regional_labels_without_addresses() -> None:
    for device in records():
        assert device["weather_zone"] == "SCENT"
        assert device["load_zone"] == "LZ_AEN"
        assert not ADDRESS_FIELDS.intersection(device)
