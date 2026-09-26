import hashlib

from tools.generation.fleet.generate import generate_fleet

EXPECTED_SHA256 = "b253a99638e14a2bd00d35c65b7c06942f9a345a65c1f7b9060cba6ff9011081"


def test_canonical_fleet_hash_is_stable() -> None:
    fleet = generate_fleet(seed=20260926, size=5000)

    assert hashlib.sha256(fleet).hexdigest() == EXPECTED_SHA256


def test_same_seed_is_byte_identical() -> None:
    assert generate_fleet(seed=20260926, size=5000) == generate_fleet(seed=20260926, size=5000)


def test_small_fleet_is_prefix_stable() -> None:
    full = generate_fleet(seed=20260926, size=5000).splitlines(keepends=True)

    assert generate_fleet(seed=20260926, size=50) == b"".join(full[:50])
