import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).parents[2]
MANIFEST_PATH = Path(__file__).with_name("MANIFEST.json")


def provenance(value: str) -> str:
    if value == "INFERRED_NOT_VERIFIED":
        raise ValueError("refusing unverified data")
    if value == "CONFIRMED_ORGANIZER_SANDBOX":
        return "CONFIRMED_SANDBOX"
    return value


def source_url(path: Path) -> str:
    relative = path.relative_to(ROOT).as_posix()
    name = path.name
    if relative.startswith("testdata/fixtures/public/fleet/"):
        return "https://www.basepowercompany.com/blog/aggregated-ders-and-the-capacity-crunch"
    if relative.startswith("data/texas/ercot-prices/"):
        report = (
            "13061"
            if name.startswith(("rtm", "RTM"))
            else "13101"
            if "system-load" in name
            else "13060"
        )
        return f"https://www.ercot.com/misapp/GetReports.do?reportTypeId={report}"
    if relative.startswith("data/texas/load-profiles/"):
        return "https://www.ercot.com/mktinfo/loadprofile/alp"
    if relative.startswith("data/texas/outages/"):
        return f"https://storage.googleapis.com/outage_data_export/{name}"
    if relative.startswith("data/texas/weather/"):
        return "https://api.weather.gov/"
    if relative.startswith("data/illinois/pjm-prices/pjm_lmp"):
        return f"https://www.eia.gov/electricity/wholesalemarkets/csv/{name}"
    if relative.startswith("data/illinois/pjm-prices/comed_"):
        return "https://hourlypricing.comed.com/api"
    if relative.startswith("data/illinois/load-profiles/"):
        return "s3://oedi-data-lake/nrel-pds-building-stock/end-use-load-profiles-for-us-building-stock/2021/resstock_tmy3_release_1/timeseries_aggregates/by_state/state=IL/"
    if name.startswith("eia930_"):
        return f"https://www.eia.gov/electricity/gridmonitor/sixMonthFiles/{name.upper()}"
    if name.startswith(("comed_hourly", "pjm_hourly")):
        return "https://www.eia.gov/electricity/gridmonitor/"
    if name == "comed_ratebook.pdf":
        return "https://azure-na-assets.contentstack.com/v3/assets/blt3ebb3fed6084be2a/blt86ebee5fe6ed02f8/Ratebook.pdf"
    if name.startswith("comed_load_forecast"):
        return "https://ipa.illinois.gov/"
    if relative.startswith("data/illinois/weather/"):
        return "https://api.weather.gov/"
    raise ValueError(f"no source URL for {relative}")


DATE_RULES = (
    (("ercot-system-load-merged",), "2026-09-14/2026-09-24"),
    (("2024_h1", "2024_jan_jun"), "2024-01-01/2024-06-30"),
    (("2024_h2", "2024_jul_dec"), "2024-07-01/2024-12-31"),
    (("2025Q1",), "2025-01-01/2025-03-31"),
    (("2025Q2", "2025_h1", "2025_jan_jun"), "2025-01-01/2025-06-30"),
    (("2025_h2", "2025_jul_dec"), "2025-07-01/2025-12-31"),
    (("2026_h1", "2026_jan_jun"), "2026-01-01/2026-06-30"),
    (("2021",), "2021-01-01/2021-12-31"),
    (("2022",), "2022-01-01/2022-12-31"),
    (("2023",), "2023-01-01/2023-12-31"),
    (("2024",), "2024-01-01/2024-12-31"),
    (("2025",), "2025-01-01/2025-12-31"),
    (("2026",), "2026-01-01/2026-09-25"),
    (("outage",), "2021-01-01/2023-12-20"),
    (("forecast", "alerts"), "2026-09-25"),
    (("il-",), "2018-01-01/2019-01-01"),
    ((".pdf",), "as published"),
    (("comed_",), "capture-specific"),
)


def date_range(path: Path) -> str:
    name = path.name
    if path.is_relative_to(ROOT / "testdata/fixtures/public/fleet"):
        return "as published"
    for fragments, result in DATE_RULES:
        if any(fragment in name for fragment in fragments):
            return result
    raise ValueError(f"no date range for {name}")


def cached_files() -> list[Path]:
    roots = (ROOT / "data", ROOT / "testdata/fixtures/public/fleet")
    return sorted(
        path
        for root in roots
        for path in root.rglob("*")
        if path.is_file() and path.name != "PROVENANCE.md"
    )


def build_manifest() -> dict[str, object]:
    files = []
    for path in cached_files():
        files.append(
            {
                "date_range": date_range(path),
                "path": path.relative_to(ROOT).as_posix(),
                "provenance": provenance("CONFIRMED_PUBLIC"),
                "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                "size_bytes": path.stat().st_size,
                "source_url": source_url(path),
            }
        )
    return {"schema_version": 1, "files": files}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    rendered = json.dumps(build_manifest(), indent=2, sort_keys=True) + "\n"
    if args.check:
        if MANIFEST_PATH.read_text() != rendered:
            raise SystemExit("manifest is stale")
        return
    MANIFEST_PATH.write_text(rendered)


if __name__ == "__main__":
    main()
