import hashlib

from tools.generation.fleet.generate import generate_fleet

EXPECTED_SHA256 = "06988268ad6efa6b93efbf9a1f29b82554584eaecf49caabadb12ce91dae4097"


def test_canonical_fleet_hash_is_stable() -> None:
    fleet = generate_fleet(seed=20260926, size=5000)

    assert hashlib.sha256(fleet).hexdigest() == EXPECTED_SHA256


def test_same_seed_is_byte_identical() -> None:
    assert generate_fleet(seed=20260926, size=5000) == generate_fleet(seed=20260926, size=5000)


def test_small_fleet_is_prefix_stable() -> None:
    full = generate_fleet(seed=20260926, size=5000).splitlines(keepends=True)

    assert generate_fleet(seed=20260926, size=50) == b"".join(full[:50])
