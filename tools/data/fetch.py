import argparse
import hashlib
import io
import json
import re
import time
import urllib.parse
import urllib.request
import zipfile
from collections.abc import Callable
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).parents[2]
MANIFEST = Path(__file__).with_name("MANIFEST.json")


def get_url(url: str) -> bytes:
    error = None
    for attempt in range(3):
        try:
            with urllib.request.urlopen(url, timeout=20) as response:
                return response.read()
        except OSError as caught:
            error = caught
            if attempt < 2:
                time.sleep(2**attempt)
    raise RuntimeError(f"download failed: {url}") from error


def fetch_ercot_system_load(
    entry: dict[str, object], get: Callable[[str], bytes] = get_url
) -> bytes:
    report_url = str(entry["source_url"])
    page = get(report_url).decode("latin-1")
    matches = re.findall(r"(cdr\.[^<]*_csv\.zip).*?href='([^']*doclookupId=\d+)'", page)
    start_text, end_text = str(entry["date_range"]).split("/", 1)
    start = datetime.fromisoformat(start_text).date()
    end = datetime.fromisoformat(end_text).date()
    header = None
    rows = []
    for _, relative_url in sorted(matches):
        archive_bytes = get(urllib.parse.urljoin(report_url, relative_url))
        with zipfile.ZipFile(io.BytesIO(archive_bytes)) as archive:
            lines = archive.read(archive.namelist()[0]).decode().splitlines()
        header = header or lines[0]
        for line in lines[1:]:
            day = datetime.strptime(line.split(",", 1)[0], "%m/%d/%Y").date()
            if start <= day <= end:
                rows.append(line)
    if header is None or not rows:
        raise RuntimeError("ERCOT report page did not contain the requested dates")
    return ("\n".join([header, *rows]) + "\n").encode()


def entry_id(entry: dict[str, object]) -> str:
    name = Path(str(entry["path"])).stem
    if name == "ercot-system-load-merged":
        return "ercot-system-load"
    return name


def fetch_entry(entry: dict[str, object]) -> bytes:
    if entry_id(entry) == "ercot-system-load":
        return fetch_ercot_system_load(entry)
    return get_url(str(entry["source_url"]))


def verify_entry(entry: dict[str, object], payload: bytes) -> None:
    actual = hashlib.sha256(payload).hexdigest()
    expected = str(entry["sha256"])
    if actual != expected:
        raise RuntimeError(f"checksum mismatch for {entry_id(entry)}: {actual}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--only")
    parser.add_argument("--verify", action="store_true")
    args = parser.parse_args()
    manifest = json.loads(MANIFEST.read_text())
    entries = [
        entry for entry in manifest["files"] if args.only is None or entry_id(entry) == args.only
    ]
    if not entries:
        raise SystemExit(f"manifest entry not found: {args.only}")
    for entry in entries:
        payload = fetch_entry(entry)
        verify_entry(entry, payload)
        destination = ROOT / str(entry["path"])
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(payload)
        print(f"verified {entry_id(entry)} {entry['sha256']}")


if __name__ == "__main__":
    main()
