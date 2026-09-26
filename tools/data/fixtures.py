import csv
import shutil
from collections.abc import Callable
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).parents[2]
PUBLIC = ROOT / "testdata/fixtures/public"


PROVENANCE = {
    "fleet": "Provenance: CONFIRMED_PUBLIC\n\nSource: https://www.basepowercompany.com/blog/aggregated-ders-and-the-capacity-crunch\n",
    "ercot-prices": "Provenance: CONFIRMED_PUBLIC\n\nSource: https://www.ercot.com/misapp/GetReports.do\n\nSelection: January 1 through January 7, 2025 for LZ_HOUSTON and HB_HOUSTON.\n",
    "load-profiles": "Provenance: CONFIRMED_PUBLIC\n\nSource: https://www.ercot.com/mktinfo/loadprofile/alp\n\nSelection: January 1 through January 7, 2025 for all 32 residential profile types.\n",
    "outages": "Provenance: DERIVED\n\nSource: https://storage.googleapis.com/outage_data_export/texas_outage_event_data.csv\n\nLineage: Customer-minute weighted monthly outage rates for Travis and Harris counties in January 2023.\n",
    "weather": "Provenance: CONFIRMED_PUBLIC\n\nSource: https://api.weather.gov/\n\nSelection: Forecast and alert snapshots captured September 25, 2026 for Austin, Dallas, Houston, and San Antonio.\n",
    "system-load": "Provenance: CONFIRMED_PUBLIC\n\nSource: https://www.ercot.com/misapp/GetReports.do?reportTypeId=13101\n\nSelection: Complete merged research-cache snapshot from September 14 through September 24, 2026.\n",
}


def write_csv(source: Path, destination: Path, include: Callable[[dict[str, str]], bool]) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    with (
        source.open(newline="") as input_stream,
        destination.open("w", newline="") as output_stream,
    ):
        reader = csv.DictReader(input_stream)
        writer = csv.DictWriter(output_stream, fieldnames=reader.fieldnames)
        writer.writeheader()
        writer.writerows(row for row in reader if include(row))


def price_fixtures() -> None:
    source_dir = ROOT / "data/texas/ercot-prices"
    destination = PUBLIC / "ercot-prices"

    def selected(row: dict[str, str]) -> bool:
        point = row.get("Settlement Point") or row["Settlement Point Name"]
        day = datetime.strptime(row["Delivery Date"], "%m/%d/%Y").date()
        return point in {"LZ_HOUSTON", "HB_HOUSTON"} and day.day <= 7 and day.month == 1

    write_csv(source_dir / "dam-spp-2025.csv", destination / "dam-spp-week.csv", selected)
    write_csv(source_dir / "rtm-spp-2025.csv", destination / "rtm-spp-week.csv", selected)


def profile_fixtures() -> None:
    source = ROOT / "data/texas/load-profiles/load-profiles-residential-2025.csv"

    def selected(row: dict[str, str]) -> bool:
        day = datetime.fromisoformat(row["Date"]).date()
        return day.month == 1 and day.day <= 7

    write_csv(source, PUBLIC / "load-profiles/residential-week.csv", selected)


def outage_fixtures() -> None:
    source = ROOT / "data/texas/outages/texas_outage_event_data.csv"
    totals: dict[str, list[float]] = {"Travis": [0.0, 0.0], "Harris": [0.0, 0.0]}
    with source.open(newline="") as stream:
        for row in csv.DictReader(stream):
            county = row["county"]
            if county not in totals or not row["min_updated_time"].startswith("2023-01"):
                continue
            duration = float(row["outage_duration_mins"])
            totals[county][0] += int(row["customers_out"]) * duration
            totals[county][1] += int(row["customers_tracked"]) * duration
    destination = PUBLIC / "outages/outage-rates-2023-01.csv"
    destination.parent.mkdir(parents=True, exist_ok=True)
    with destination.open("w", newline="") as stream:
        writer = csv.writer(stream)
        writer.writerow(["county", "month", "outage_rate", "provenance"])
        for county, values in totals.items():
            writer.writerow([county, "2023-01", values[0] / values[1], "DERIVED"])


def copied_fixtures() -> None:
    weather = PUBLIC / "weather"
    weather.mkdir(parents=True, exist_ok=True)
    for source in (ROOT / "data/texas/weather").glob("*.json"):
        shutil.copyfile(source, weather / source.name)
    system_load = PUBLIC / "system-load"
    system_load.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(
        ROOT / "data/texas/ercot-prices/ercot-system-load-merged.csv",
        system_load / "ercot-system-load-merged.csv",
    )


def main() -> None:
    price_fixtures()
    profile_fixtures()
    outage_fixtures()
    copied_fixtures()
    for directory, content in PROVENANCE.items():
        destination = PUBLIC / directory
        destination.mkdir(parents=True, exist_ok=True)
        (destination / "PROVENANCE.md").write_text(content)


if __name__ == "__main__":
    main()
