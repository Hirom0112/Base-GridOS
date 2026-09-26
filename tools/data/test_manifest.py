import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).parents[2]


def test_manifest_covers_cached_datasets() -> None:
    manifest = json.loads((ROOT / "tools/data/MANIFEST.json").read_text())
    entries = {entry["path"]: entry for entry in manifest["files"]}
    cached = {
        str(path.relative_to(ROOT))
        for directory in (ROOT / "data", ROOT / "testdata/fixtures/public/fleet")
        for path in directory.rglob("*")
        if path.is_file() and path.name != "PROVENANCE.md"
    }
    assert entries.keys() == cached
    for relative_path, entry in entries.items():
        path = ROOT / relative_path
        assert path.stat().st_size == entry["size_bytes"]
        assert hashlib.sha256(path.read_bytes()).hexdigest() == entry["sha256"]
        assert entry["source_url"].startswith(("https://", "s3://"))
        assert entry["provenance"] in {
            "CONFIRMED_PUBLIC",
            "CONFIRMED_SANDBOX",
            "AUTHORIZED_OPERATIONAL",
            "DERIVED",
            "SIMULATED",
        }
        assert entry["date_range"]
