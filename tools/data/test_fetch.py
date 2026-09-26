import io
import zipfile

from fetch import fetch_ercot_system_load


def test_recorded_ercot_response_is_merged() -> None:
    report_url = "https://www.ercot.com/misapp/GetReports.do?reportTypeId=13101"
    download_url = (
        "https://www.ercot.com/misdownload/servlets/mirDownload?mimic_duns=000000000&doclookupId=1"
    )
    page = b"<td>cdr.00013101.20260915.ACTUALSYSLOADWZNP6345_csv.zip</td><a href='/misdownload/servlets/mirDownload?mimic_duns=000000000&doclookupId=1'>zip</a>"
    archive_stream = io.BytesIO()
    with zipfile.ZipFile(archive_stream, "w") as archive:
        archive.writestr(
            "load.csv",
            "OperDay,HourEnding,TOTAL\r\n09/14/2026,01:00,10\r\n09/13/2026,01:00,9\r\n",
        )
    responses = {report_url: page, download_url: archive_stream.getvalue()}

    payload = fetch_ercot_system_load(
        {"date_range": "2026-09-14/2026-09-14", "source_url": report_url},
        responses.__getitem__,
    )

    assert payload == b"OperDay,HourEnding,TOTAL\n09/14/2026,01:00,10\n"
