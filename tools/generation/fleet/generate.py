import hashlib
import json
import random
from pathlib import Path

import h3

WEATHER_ZONES = {
    "COAST": (28.7, 30.4, -96.2, -94.5),
    "EAST": (30.4, 33.0, -95.8, -93.6),
    "FWEST": (30.0, 32.4, -104.8, -101.4),
    "NCENT": (31.5, 34.0, -99.8, -96.2),
    "NORTH": (34.0, 36.4, -103.0, -99.0),
    "SCENT": (28.7, 31.5, -100.2, -96.2),
    "SOUTH": (25.8, 28.7, -100.0, -96.8),
    "WEST": (28.5, 31.5, -103.0, -100.2),
}
PROFILE_TYPES = ("RESHIWR", "RESHIDG", "RESHIPV", "RESHIWD")
PLAN_RESERVES = {
    "RESILIENT": (60, 70, 80),
    "BALANCED": (30, 40, 50),
    "GRID_FLEX": (0, 10, 20),
}
LOAD_ZONES = {
    "COAST": "LZ_HOUSTON",
    "EAST": "LZ_HOUSTON",
    "FWEST": "LZ_WEST",
    "NCENT": "LZ_NORTH",
    "NORTH": "LZ_NORTH",
    "SCENT": "LZ_SOUTH",
    "SOUTH": "LZ_SOUTH",
    "WEST": "LZ_WEST",
}
RELIABILITY_TRAITS = ("HIGH", "MEDIUM", "LOW")


def _identifier(kind: str, seed: int, index: int) -> str:
    digest = hashlib.sha256(f"{kind}:{seed}:{index}".encode()).hexdigest()[:20]
    return f"{kind}_{digest}"


def _device(seed: int, index: int) -> dict[str, object]:
    rng = random.Random(f"{seed}:{index}")
    weather_zone = tuple(WEATHER_ZONES)[index % len(WEATHER_ZONES)]
    latitude_min, latitude_max, longitude_min, longitude_max = WEATHER_ZONES[weather_zone]
    latitude = rng.uniform(latitude_min, latitude_max)
    longitude = rng.uniform(longitude_min, longitude_max)
    plan = tuple(PLAN_RESERVES)[rng.randrange(len(PLAN_RESERVES))]
    profile = PROFILE_TYPES[rng.randrange(len(PROFILE_TYPES))]
    load_zone = LOAD_ZONES[weather_zone]
    usable_energy_kwh = round(rng.uniform(10.0, 40.0), 3)
    return {
        "site_id": _identifier("site", seed, index),
        "device_id": _identifier("device", seed, index),
        "usable_energy_kwh": usable_energy_kwh,
        "max_charge_kw": round(rng.uniform(5.0, min(12.0, usable_energy_kwh)), 3),
        "max_discharge_kw": round(rng.uniform(5.0, min(12.0, usable_energy_kwh)), 3),
        "charge_efficiency": round(rng.uniform(0.92, 0.97), 4),
        "discharge_efficiency": round(rng.uniform(0.92, 0.97), 4),
        "weather_zone": weather_zone,
        "load_zone": load_zone,
        "load_profile_type": f"{profile}_{weather_zone}",
        "has_solar": profile in {"RESHIDG", "RESHIPV"},
        "has_automatic_backup": index % 3 != 0,
        "h3_cell": h3.latlng_to_cell(latitude, longitude, 7),
        "cohorts": [weather_zone, load_zone, plan],
        "reliability_trait": RELIABILITY_TRAITS[rng.randrange(len(RELIABILITY_TRAITS))],
        "resilience_plan": plan,
        "reserve_preference_percent": rng.choice(PLAN_RESERVES[plan]),
        "provenance": "SIMULATED",
        "simulation_seed": seed,
    }


def generate_fleet(seed: int, size: int) -> bytes:
    if size < 0:
        raise ValueError("size must be nonnegative")
    records = (
        json.dumps(_device(seed, index), sort_keys=True, separators=(",", ":"))
        for index in range(size)
    )
    return "".join(f"{record}\n" for record in records).encode()


def write_fleet(name: str, seed: int, size: int, directory: Path) -> Path:
    if not name or Path(name).name != name:
        raise ValueError("name must be a filename stem")
    output = directory / f"{name}.jsonl"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(generate_fleet(seed, size))
    return output
