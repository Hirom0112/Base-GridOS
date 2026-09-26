# Base GridOS — Dataset Manifest

**Last verified:** September 25, 2026
**Total on disk:** ~808 MB (771 MiB), split between the ignored `data/`
research cache and checked-in candidates under `testdata/fixtures/`.
**Provenance key:** Every dataset is labeled `CONFIRMED_PUBLIC` (freely available from a named public source), `CONFIRMED_ORGANIZER_SANDBOX` (provided by hackathon organizers), or `SIMULATED` (synthetic/illustrative, not real operational data).

---

## 1. Base Power Fleet Data

**Location:** `testdata/fixtures/public/fleet/`
**Source:** Chart data published on the public Base Power blog.
**Provenance:** `CONFIRMED_PUBLIC` — published on the public Base Power blog, embedded in chart components on [/blog/aggregated-ders-and-the-capacity-crunch](https://www.basepowercompany.com/blog/aggregated-ders-and-the-capacity-crunch) and [/blog/self-scheduling](https://www.basepowercompany.com/blog/self-scheduling).

| File | Records | Size | Description |
|------|---------|------|-------------|
| `fleet_capacity_monthly.json` | 16 months | 4 KB | Monthly fleet nameplate discharge capacity (MW) with ADER partition breakdown (North, South, Houston) and ADER share percentage. May 2025 through Aug 2026. |
| `ader_dispatch_scoring.json` | 36 intervals + metadata | 12 KB | CLREDP dispatch scoring for the `lz-houston-ader` partition. Five-minute intervals from 21:00 to 00:00 CT. Fields: `s`, `ws`, `we`, `realized`, `reference`, `tolerance`, `deviation`, `outside`, `label`. Includes summary constants (max discharge/charge MW, tolerance, mean/max deviation). |
| `sced_series.json` | 361 data points | 36 KB | High-resolution SCED dispatch response at 30-second cadence over a 3-hour window. Fields per point: `s` (seconds), `realized` (fleet power MW), `issued` (set point MW), `sced` (ERCOT SCED base point MW). |
| `state_estimator_line_flows.json` | 2 buses, 109 telemetry points each | 20 KB | ERCOT 60-day state-estimator line flows at two 138 kV buses on January 16, 2026. Per bus: 9 SE hourly readings, 9 flanking-day baseline readings, 109 battery 5-min telemetry points, 4 block annotations. Fields per bus: `bus`, `swingUpMw`, `swingDownMw`, `ratio`, `se[]`, `baseline[]`, `battery[]`, `blocks[]`. |
| `ptdf_constraint_analysis.json` | 1 monitored element, 2 substations | 4 KB | Transmission constraint analysis for monitored element `5127__A`. Emergency rating 1,200 MVA. 40 MW of sited ADER at WILLOW BEND (PTDF 0.88) and PECAN FLAT (PTDF 0.83) relieving 34.2 MW of a 32 MW overload. Includes 10-row summary table. |
| `eia_projections.json` | 77 years history + 11 scenarios | 16 KB | US electricity consumption from EIA Annual Energy Outlook 2026. History: 1949 (254.5 TWh) through 2025 (4,195.1 TWh). Projections: 11 named AEO2026 cases through 2036 (range: 4,612–5,054 TWh). Fields: `year`, `twh` (history); case name and band values (projections). |

---

## 2. Texas — ERCOT Prices

**Location:** `data/texas/ercot-prices/`
**Provenance:** `CONFIRMED_PUBLIC`

### Day-Ahead Settlement Point Prices (NP4-180-ER)

**Source:** ERCOT MIS, report type 13060 — `https://www.ercot.com/misapp/GetReports.do?reportTypeId=13060`

| File | Rows | Size | Date Range | Resolution |
|------|------|------|------------|------------|
| `dam-spp-2025.csv` | 131,400 | 4.7 MB | 01/01/2025 – 12/31/2025 | Hourly |
| `dam-spp-2026.csv` | 94,305 | 3.7 MB | 01/01/2026 – 09/19/2026 | Hourly |

**Columns:** `Delivery Date`, `Hour Ending`, `Repeated Hour Flag`, `Settlement Point`, `Settlement Point Price`
**Settlement points (15):** HB_BUSAVG, HB_HOUSTON, HB_HUBAVG, HB_NORTH, HB_PAN, HB_SOUTH, HB_WEST, LZ_AEN, LZ_CPS, LZ_HOUSTON, LZ_LCRA, LZ_NORTH, LZ_RAYBN, LZ_SOUTH, LZ_WEST

### Real-Time Settlement Point Prices (NP6-785-ER)

**Source:** ERCOT MIS, report type 13061 — `https://www.ercot.com/misapp/GetReports.do?reportTypeId=13061`

| File | Rows | Size | Date Range | Resolution |
|------|------|------|------------|------------|
| `rtm-spp-2025.csv` | 805,920 | 29 MB | 01/01/2025 – 12/31/2025 | 15-minute |
| `rtm-spp-2026.csv` | 578,404 | 21 MB | 01/01/2026 – 09/19/2026 | 15-minute |

**Columns:** `Delivery Date`, `Delivery Hour`, `Delivery Interval`, `Repeated Hour Flag`, `Settlement Point Name`, `Settlement Point Type`, `Settlement Point Price`
**Settlement points:** Same 15 as DAM.

### System Load by Weather Zone (NP6-345-CD)

**Source:** ERCOT MIS, report type 13101 — `https://www.ercot.com/misapp/GetReports.do?reportTypeId=13101`

| File | Rows | Size | Date Range | Resolution |
|------|------|------|------------|------------|
| `ercot-system-load-merged.csv` | 264 | 25 KB | 09/14/2026 – 09/24/2026 | Hourly |

**Columns:** `OperDay`, `HourEnding`, `COAST`, `EAST`, `FAR_WEST`, `NORTH`, `NORTH_C`, `SOUTHERN`, `SOUTH_C`, `WEST`, `TOTAL`, `DSTFlag`
**Values in MW.** Note: this product only retains 31 days on MIS. The merged
snapshot retains the 264 data rows from the 11 downloaded daily files; the
redundant individual files are not retained.

**Also on disk:** Original ZIP archives for DAM and RTM data
(`DAMLZHBSPP_2025.zip`, `DAMLZHBSPP_2026.zip`, `RTMLZHBSPP_2025.zip`, and
`RTMLZHBSPP_2026.zip`). Their extracted XLSX members were verified byte-for-byte
against the archives and are not stored a second time.

---

## 3. Texas — Residential Load Profiles

**Location:** `data/texas/load-profiles/`
**Source:** ERCOT Backcasted Load Profiles — `https://www.ercot.com/mktinfo/loadprofile/alp`
**Provenance:** `CONFIRMED_PUBLIC`

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `load-profiles-residential-2025.csv` | 12,608 | 8 MB | 01/01/2025 – 12/31/2025 |
| `load-profiles-residential-2026.csv` | 6,752 | 4 MB | 01/01/2026 – 08/30/2026 |

**Columns (103):** `PType_WZ`, `Date`, `int_kWh1` through `int_kWh100`, `ADDTIME`
- 96 intervals = 15-minute kWh values per day (4 extra columns for DST long days).
- 32 residential profile types across 8 weather zones.

**Profile types:** RESHIWR (residential without DG/PV/wind), RESHIDG (with distributed generation), RESHIPV (with PV), RESHIWD (with wind) — each for weather zones COAST, EAST, FWEST, NCENT, NORTH, SCENT, SOUTH, WEST.

**Also on disk:** Source ZIP archives containing the original XLSX workbooks
with all 248 profile types (residential and business/commercial). Extracted XLSX
copies were verified against the archive members and removed as duplicates.

---

## 4. Texas — Outage Data

**Location:** `data/texas/outages/`
**Source:** Base Power public GCS bucket — `https://storage.googleapis.com/outage_data_export/`
**Provenance:** `CONFIRMED_PUBLIC` — hosted by Base Power for the 2024 Austin hackathon; bucket remains publicly accessible.

| File | Rows | Size | Description |
|------|------|------|-------------|
| `texas_outage_event_data.csv` | 754,216 events | 81 MB | Aggregated outage events. Full download. |
| `outage_data_sample_1000.csv` | 1,000 | 92 KB | Sample of the raw time-series file (full: 269 MB remote). |
| `hackathon_outage_data_sample_1000.csv` | 1,000 | 92 KB | Sample of the hackathon-labeled raw file (full: 269 MB remote). |

**Events file columns:** `outage_id`, `utility`, `county`, `min_updated_time`, `max_updated_time`, `customers_tracked`, `customers_out`, `outage_duration_mins`
**Raw time-series columns:** `utility`, `county`, `city`, `customers_tracked`, `customers_out`, `updated_datetime`
**Date range:** 2021-01-01 through 2023-12-20
**Utilities:** 66 Texas utilities including Oncor, CenterPoint, Austin Energy, CPS Energy, AEP Texas, Entergy, TNMP, El Paso Electric, and 58 others.
**Top counties by events:** Travis (29,142), Montgomery (11,482), Williamson (10,780), Bexar (10,623), Liberty (10,270).

---

## 5. Texas — Weather

**Location:** `data/texas/weather/`
**Source:** National Weather Service API — `https://api.weather.gov/`
**Provenance:** `CONFIRMED_PUBLIC`

| File | Size | Description |
|------|------|-------------|
| `austin_forecast.json` | 14 KB | 7-day forecast for Austin (30.27, -97.74) |
| `austin_alerts.json` | <1 KB | Active weather alerts |
| `houston_forecast.json` | 14 KB | 7-day forecast for Houston (29.76, -95.37) |
| `houston_alerts.json` | <1 KB | Active alerts (Air Quality Alert as of capture) |
| `dallas_forecast.json` | 14 KB | 7-day forecast for Dallas (32.78, -96.80) |
| `dallas_alerts.json` | <1 KB | Active alerts (Air Quality Alert as of capture) |
| `san_antonio_forecast.json` | 13 KB | 7-day forecast for San Antonio (29.42, -98.49) |
| `san_antonio_alerts.json` | <1 KB | Active alerts |

**Format:** GeoJSON per NWS API spec. Forecasts include temperature, wind speed/direction, precipitation probability, and short/detailed text forecasts in 12-hour periods.
**Snapshot date:** September 25, 2026. These are point-in-time snapshots; re-fetch for current data.

---

## 6. Illinois — PJM/ComEd Prices

**Location:** `data/illinois/pjm-prices/`
**Provenance:** `CONFIRMED_PUBLIC`

### PJM Day-Ahead Hourly LMPs (all zones)

**Source:** EIA Wholesale Electricity Market Portal — `https://www.eia.gov/electricity/wholesalemarkets/csv/pjm_lmp_da_hr_zones_{YYYY}.csv`

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `pjm_lmp_da_hr_zones_2021.csv` | 8,764 | 6.9 MB | Full 2021 |
| `pjm_lmp_da_hr_zones_2022.csv` | 8,740 | 7.5 MB | Full 2022 |
| `pjm_lmp_da_hr_zones_2023.csv` | 7,588 | 6.0 MB | Partial 2023 |
| `pjm_lmp_da_hr_zones_2024.csv` | 8,236 | 6.5 MB | Full 2024 |
| `pjm_lmp_da_hr_zones_2025.csv` | 4,203 | 3.3 MB | Jan–Jun 2025 |

**Columns (88):** Hourly LMP, Congestion, Energy, and Loss components for all 22 PJM zones. ComEd LMP is column 11.
**Resolution:** Hourly.

### PJM Real-Time Hourly LMPs (all zones)

**Source:** `https://www.eia.gov/electricity/wholesalemarkets/csv/pjm_lmp_rt_hr_zones_{YYYY}.csv`

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `pjm_lmp_rt_hr_zones_2021.csv` | 8,764 | 7.0 MB | Full 2021 |
| `pjm_lmp_rt_hr_zones_2022.csv` | 8,764 | 8.0 MB | Full 2022 |
| `pjm_lmp_rt_hr_zones_2023.csv` | 7,396 | 5.9 MB | Partial 2023 |
| `pjm_lmp_rt_hr_zones_2024.csv` | 8,212 | 7.3 MB | Full 2024 |
| `pjm_lmp_rt_hr_zones_2025.csv` | 4,155 | 3.3 MB | Jan–Jun 2025 |

### PJM Real-Time 5-Minute LMPs (all zones)

**Source:** `https://www.eia.gov/electricity/wholesalemarkets/csv/pjm_lmp_rt_5min_zones_{YYYY}Q{N}.csv`

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `pjm_lmp_rt_5min_zones_2025Q1.csv` | 25,912 | 21 MB | Jan–Mar 2025 |
| `pjm_lmp_rt_5min_zones_2025Q2.csv` | 24,124 | 19 MB | Apr–Jun 2025 |

### ComEd Hourly Pricing API (5-minute prices)

**Source:** ComEd Hourly Pricing API — `https://hourlypricing.comed.com/api?type=5minutefeed&datestart=YYYYMMDD0000&dateend=YYYYMMDD0000`
**Access:** Public, no authentication required. Supports historical queries back to at least September 2025.

| File | Records | Description |
|------|---------|-------------|
| `comed_5min_sept2025.json` | ~8,598 | September 2025 5-min prices |
| `comed_5min_jan2026.json` | ~8,886 | January 2026 5-min prices |
| `comed_5min_jul2026.json` | ~8,784 | July 2026 5-min prices |
| `comed_5min_sept2026.json` | ~6,764 | September 1–25, 2026 5-min prices |
| `comed_5min_prices_current.json` | ~264 | Last ~22 hours at capture time |
| `comed_dayahead_prices_today.json` | 24 | Today's day-ahead hourly prices |
| `comed_current_hour_avg.json` | 1 | Current hour average |

**Format:** JSON array of objects with `millisUTC` (Unix ms timestamp) and `price` (cents/kWh).

---

## 7. Illinois — Residential Load Profiles

**Location:** `data/illinois/load-profiles/`
**Source:** NREL End-Use Load Profiles for the US Building Stock, ResStock TMY3 Release 1 — `s3://oedi-data-lake/nrel-pds-building-stock/end-use-load-profiles-for-us-building-stock/2021/resstock_tmy3_release_1/timeseries_aggregates/by_state/state=IL/`
**Documentation:** `https://www.nrel.gov/buildings/end-use-load-profiles.html`
**Provenance:** `CONFIRMED_PUBLIC` — free via AWS S3 (OEDI data lake), no account required.
**License:** ResStock open-source (see NREL GitHub for terms).

| File | Rows | Size | Building Type |
|------|------|------|--------------|
| `il-single-family_detached.csv` | 35,041 | 31 MB | Single-Family Detached |
| `il-single-family_attached.csv` | 35,041 | 26 MB | Single-Family Attached |
| `il-multi-family_with_2_-_4_units.csv` | 35,041 | 27 MB | 2–4 Unit Multi-Family |
| `il-multi-family_with_5plus_units.csv` | 35,041 | 27 MB | 5+ Unit Multi-Family |
| `il-mobile_home.csv` | 35,041 | 26 MB | Mobile Home |

**Resolution:** 15-minute intervals for a full typical meteorological year (2018-01-01 through 2019-01-01).
**Key columns (59):** `in.state`, `in.geometry_building_type_recs`, `timestamp`, `models_used`, `units_represented`, then end-use electricity consumption breakdowns: cooling, heating, fans, lighting, plug loads, refrigeration, water heating, clothes dryer/washer, cooking, EV charging, pool/hot tub, PV generation, and more.
**Coverage:** Illinois statewide, aggregated from calibrated building energy simulation models representing the actual Illinois housing stock.

---

## 8. Illinois — Demand and Grid Data

**Location:** `data/illinois/`
**Provenance:** `CONFIRMED_PUBLIC`

### EIA-930 Balance Files (all US balancing authorities)

**Source:** EIA Grid Monitor — `https://www.eia.gov/electricity/gridmonitor/sixMonthFiles/EIA930_BALANCE_{YYYY}_{period}.csv`

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `eia930_balance_2024_jan_jun.csv` | ~260K | 40 MB | Jan–Jun 2024 |
| `eia930_balance_2024_jul_dec.csv` | ~260K | 46 MB | Jul–Dec 2024 |
| `eia930_balance_2025_jan_jun.csv` | ~260K | 45 MB | Jan–Jun 2025 |
| `eia930_balance_2025_jul_dec.csv` | ~260K | 47 MB | Jul–Dec 2025 |
| `eia930_balance_2026_jan_jun.csv` | ~260K | 45 MB | Jan–Jun 2026 |

**Key columns (65+):** `Balancing Authority`, `Data Date`, `Hour Number`, `Demand Forecast (MW)`, `Demand (MW)`, `Net Generation (MW)`, `Total Interchange (MW)`, generation by fuel type (Coal, Natural Gas, Nuclear, Solar, Wind, Battery, Hydro, etc.).
**Coverage:** All US balancing authorities. Filter on `PJM` for PJM-wide or see subregion files for ComEd zone.

### EIA-930 Subregion Files

**Source:** `https://www.eia.gov/electricity/gridmonitor/sixMonthFiles/EIA930_SUBREGION_{YYYY}_{period}.csv`

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `eia930_subregion_2024_jul_dec.csv` | ~260K | 26 MB | Jul–Dec 2024 |
| `eia930_subregion_2025_jan_jun.csv` | ~260K | 25 MB | Jan–Jun 2025 |
| `eia930_subregion_2026_jan_jun.csv` | ~260K | 26 MB | Jan–Jun 2026 |

**Filter `Sub-Region = CE`** to get ComEd zone hourly demand.

### ComEd Zone Hourly Demand (extracted from EIA-930 subregion)

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `comed_hourly_demand_2024_h2.csv` | 4,418 | 308 KB | Jul–Dec 2024 |
| `comed_hourly_demand_2025_h1.csv` | 4,344 | 300 KB | Jan–Jun 2025 |
| `comed_hourly_demand_2026_h1.csv` | 4,344 | 300 KB | Jan–Jun 2026 |

**Columns:** `Balancing Authority`, `Data Date`, `Hour Number`, `Sub-Region`, `Demand (MW)`, `Local Time at End of Hour`, `UTC Time at End of Hour`

### PJM RTO-Level Hourly Demand (with generation by fuel type)

**Source:** Extracted from EIA-930 balance files, filtered to `Balancing Authority = PJM`.

| File | Rows | Size | Date Range |
|------|------|------|------------|
| `pjm_hourly_demand_2024_h1.csv` | 4,368 | 888 KB | Jan–Jun 2024 |
| `pjm_hourly_demand_2024_h2.csv` | 4,418 | 988 KB | Jul–Dec 2024 |
| `pjm_hourly_demand_2025_h1.csv` | 4,344 | 972 KB | Jan–Jun 2025 |
| `pjm_hourly_demand_2025_h2.csv` | 4,418 | 988 KB | Jul–Dec 2025 |
| `pjm_hourly_demand_2026_h1.csv` | 4,344 | 976 KB | Jan–Jun 2026 |

**Columns (65):** Includes Demand Forecast, Demand, Net Generation, Total Interchange, and generation by fuel type (Coal, Natural Gas, Nuclear, Solar, Wind, Battery, etc.) in MW.

### ComEd Reference Documents

| File | Size | Description |
|------|------|-------------|
| `comed_ratebook.pdf` | 13 MB | Full ComEd schedule of rates (~800 pages, all tariffs) |
| `comed_load_forecast_2025.pdf` | 1.7 MB | ComEd 5-year load forecast (IPA filing) |
| `comed_load_forecast_2026.pdf` | 1.8 MB | ComEd 5-year load forecast, latest (IPA filing) |

**Source (rate book):** `https://azure-na-assets.contentstack.com/v3/assets/blt3ebb3fed6084be2a/blt86ebee5fe6ed02f8/Ratebook.pdf`
**Source (forecasts):** IPA procurement plan documents at `https://ipa.illinois.gov/`

---

## 9. Illinois — Weather

**Location:** `data/illinois/weather/`
**Source:** National Weather Service API — `https://api.weather.gov/`
**Provenance:** `CONFIRMED_PUBLIC`

| File | Size | Description |
|------|------|-------------|
| `chicago_forecast.json` | 13 KB | 7-day forecast for Chicago (41.88, -87.63) |
| `chicago_alerts.json` | <1 KB | Active weather alerts at time of capture |

**Snapshot date:** September 25, 2026.

---

## 10. Not Yet Downloaded — Available Free

These datasets have been identified and verified as publicly accessible. Download instructions are included.

### Grid Topology

| Dataset | Source | Size (est.) | Description | How to Get |
|---------|--------|------------|-------------|------------|
| **HIFLD Electric Substations** | `https://hifld-geoplatform.opendata.arcgis.com/datasets/electric-substations` | ~50 MB | ~67,000 US substations with lat/lon, name, voltage, owner. Shapefile/GeoJSON/CSV. Filter to TX and IL. | Direct download from HIFLD ArcGIS Hub |
| **HIFLD Transmission Lines** | `https://hifld-geoplatform.opendata.arcgis.com/datasets/geoplatform::electric-power-transmission-lines/about` | ~200 MB | US transmission lines >= 100V with SUB_1/SUB_2 connecting substations. Shapefile/GeoJSON. | Direct download from HIFLD ArcGIS Hub |
| **PUCT Substation Coordinates** | `https://ftp.puc.texas.gov/public/puct-info/industry/electric/forms/transconsrt/Substation_Locational_Coordinate_Point_Data_Attributes.xls` | ~5 MB | Texas substations with names, lat/lon, ownership. Excel. | Direct download from PUCT FTP |
| **NREL SMART-DS Austin Feeders** | `https://data.openei.org/submissions/2981` / AWS S3 | Subset of 5.32 TB | 2,711 synthetic distribution feeders for Austin TX. Complete OpenDSS models with bus coordinates, line impedances, transformer ratings, 15-min load timeseries. | AWS S3 `oedi-data-lake` (free, no account) |
| **ComEd Hosting Capacity Map** | `https://www.comed.com/smart-energy/my-green-power-connection/developers-contractors/smaller-generators` | ~20 MB | Distribution circuit-level hosting capacity for ComEd territory. Substation names, feeder IDs, existing generation, capacity (MW). ArcGIS REST API. PV layer ID: `d282a890afb34956a906ae224c9f708e`, BESS layer ID: `e357a47d16bf4f9380855981301a644d`. | Query ArcGIS REST API or download attribute table |
| **Texas A&M ACTIVSg2000** | `https://electricgrids.engr.tamu.edu/electric-grid-test-cases/activsg2000/` | ~50 MB | 2,000-bus synthetic transmission system on the geographic footprint of Texas. Lat/lon for all buses. PowerWorld/Matpower/PSS/E formats. | Free registration required; also on GitHub at `https://github.com/caseformat/ACTIVSg2000/` |
| **Texas A&M Combined T+D** | `https://electricgrids.engr.tamu.edu/combined-td-synthetic-dataset/` | varies | Combined transmission AND distribution synthetic models for Texas. Includes feeder-level topology attached to transmission buses. | Free registration required |
| **PJM Public Network Model** | `https://www.pjm.com/markets-and-operations/energy/lmp-model-info.aspx` | ~100 MB | Bus-branch network model in PSS/E .RAW format. Includes all PJM buses (ComEd zone included), branch impedances, ratings. | Direct download from PJM |
| **PNNL Taxonomy Feeders** | `https://github.com/GRIDAPPSD/Powergrid-Models` | ~20 MB | 24 prototypical US distribution feeders. CIM XML / GridLAB-D / OpenDSS formats. Not geolocated but representative of climate zones. | GitHub clone |
| **EIA Form 860 (Power Plants)** | `https://www.eia.gov/electricity/data/eia860/` | ~50 MB | Plant-level data with lat/lon, nameplate capacity, fuel type. All US plants >= 1 MW. Excel in ZIP. | Direct download |
| **EIA Service Territories (Form 861)** | `https://atlas.eia.gov/datasets/geoplatform::electric-retail-service-territories-2` | ~30 MB | Utility service territory polygons. Oncor, CenterPoint, ComEd, etc. Shapefile/GeoJSON. | Direct download from EIA Energy Atlas |

### Outage Data (full raw files)

| Dataset | Source | Size | Description |
|---------|--------|------|-------------|
| **outage_data.csv** | `https://storage.googleapis.com/outage_data_export/outage_data.csv` | 269 MB | Full raw time-series outage observations. Same schema as the 1,000-line sample already on disk. |
| **20240920_austin_hackathon_outage_data.csv** | `https://storage.googleapis.com/outage_data_export/20240920_austin_hackathon_outage_data.csv` | 269 MB | Same data as above (appears identical, missing header row). |

### Home Battery/Energy Telemetry

| Dataset | Source | Size | Description | Battery Data? |
|---------|--------|------|-------------|---------------|
| **Pecan Street Kaggle Sample** | Kaggle (search "Pecan Street") | ~10 MB | 10 Austin TX homes, 3 August days, 1-minute circuit-level electricity. Immediate free download. | Possibly |
| **Tsukuba Microgrid Battery** | figshare (via `https://pmc.ncbi.nlm.nih.gov/articles/PMC6383063/`) | 1.3 GB | Per-second battery SOC, power, voltage, current. Jan 2015 – Apr 2018 (1,210 days). Lead-acid, 326 kWh, 90 kW. CC-BY 4.0. | **Yes** — strongest public battery telemetry dataset |
| **Zenodo Battery SOC Profiles** | `https://zenodo.org/records/15394961` | 1.6 MB | PV generation, consumption, battery SOC (%), charge/discharge at 15-min resolution. Full year 2023. CC-BY 4.0. | **Yes** — SOC + charge/discharge profiles |
| **Kaggle Synthetic BMS** | Kaggle (search "Synthetic Distributed Battery Management System") | ~10 MB | Per-module sensor readings, SOC/SoH degradation, 5-min intervals, 4 battery modules. Synthetic but realistic. | **Yes** — synthetic BMS telemetry |
| **NREL End-Use Load Profiles (Texas)** | `s3://oedi-data-lake/nrel-pds-building-stock/.../by_state/state=TX/` | ~150 MB | Same format as the Illinois profiles already downloaded. 15-min, 59 end-use columns, multiple building types. | No (consumption only) |
| **ERCOT additional years** | `https://www.ercot.com/mktinfo/loadprofile/alp` | 3–47 MB/yr | Backcasted load profiles going back to 1997. Same format as files already on disk. | No |

---

## 11. Not Yet Downloaded — Registration Required

| Dataset | Source | Access | Description |
|---------|--------|--------|-------------|
| **Pecan Street Dataport (full)** | `https://www.pecanstreet.org/dataport/` | Free academic tier (75 homes); email `dataport@pecanstreet.org` for hackathon access | 1,000+ Austin TX homes, 1-second and 1-minute resolution, circuit-level, includes energy storage. The single best per-home dataset for this project. |
| **PJM Data Miner 2 API** | `https://apiportal.pjm.com/` | Free account (subscription key) | Programmatic access to all PJM datasets: bus-level LMPs, load, interconnection queues, fuel mix. |
| **EIA Open Data API** | `https://www.eia.gov/opendata/` | Free API key | Programmatic access to hourly demand by balancing authority and other EIA datasets. |
| **IEEE DataPort Datasets** | `https://ieee-dataport.org/` | Paid subscription | Multiple residential battery + smart meter datasets (PV-Battery in Yangon, Smart Data Set with 7 US homes with batteries). |
| **ComEd Anonymized AMI Data** | Contact ComEd | $900/month or academic pricing | Anonymized interval energy usage aggregated by ZIP code. 30-min resolution. Must comply with 15/15 privacy rule. |
| **UK Low Carbon London** | `https://data.london.gov.uk/dataset/smartmeter-energy-consumption-data-in-london-households-vqm0d` | Free download | 5,567 London households, half-hourly, ~10 GB. Not TX/IL but useful for synthetic profile generation. CC Attribution. |

---

## 12. Publicly Downloadable Base-Like Data

| Dataset | Years | Real or synthetic | Required fields supplied | Main gap |
|---|---:|---|---|---|
| [NextGen ACT Energy Storage Trial](https://zenodo.org/records/14885589) | 2018 | **Real** | 100 homes at five-minute resolution with household load, solar, battery charge/discharge, SOC, battery capacity, and peak power. | No feeder mapping, temperature, or health. |
| [RWTH multi-year home-storage measurements](https://zenodo.org/records/12091223) | 2015–2022 | **Real** | 21 residential systems with battery voltage, current, power, battery-pack temperature, room temperature, and long-term degradation evidence. | No feeder mapping, household load, or explicit SOC. |
| [SMART-DS](https://data.openei.org/submissions/2981) | 2016–2018 | **Synthetic** | Customer/load-to-LV-network-to-feeder-to-substation hierarchy, equipment, electrical parameters, annual 15-minute profiles, weather, PV, and battery-placement scenarios. | No measured battery SOC, temperature, health, or real customer mapping. |
| [Cornwall Local Energy Market residential dataset](https://reshare.ukdataservice.ac.uk/854578/) | 2018–2020 | **Real** | Consumption, PV, battery charge/discharge, SOC, grid import/export, voltage, frequency, equipment capacity, and weather forecasts for 100 homes. | Registration is required; no documented feeder mapping, temperature, or health. |
| [Pecan Street Dataport](https://www.pecanstreet.org/dataport/about-dataport/) | Since 2009; Austin since 2012 | **Real** | Texas household and circuit load, solar, EV, storage, and voltage measurements, subject to licensed schema and access. | Licensed access; storage coverage and SOC fields require confirmation; no public feeder mapping. |

### Best available implementation combination

Use **SMART-DS Austin 2018** for the synthetic customer-to-feeder-to-substation network. Assign real telemetry profiles from **NextGen ACT 2018** or **Cornwall 2018–2020** to anonymous SMART-DS customers. Use **RWTH 2015–2022** to calibrate temperature and degradation behavior.

This produces a credible simulation of the missing Base data structure. The join is synthetic and must be labeled `SIMULATED`; it is not a real mapping between a measured household and its actual feeder.

The exact protected dataset to pursue is the SA Power Networks/Tesla VPP combination or the Horizon Power/Wattwatchers Carnarvon dataset. The exact Base-specific dataset still requires authorized Base or organizer access.

---

## 13. Resilience Plans and Travel Flex Data

Historical market prices, grid load, weather, outage events, and public load
profiles are sufficient to backtest the gross opportunity from temporarily
lower reserve settings. They are not sufficient to claim real customer
adoption, net margin, or security outcomes.

### Missing authorized data

- Member plan selection, informed consent, effective dates, and opt-out history.
- Travel start/end windows, early returns, and notification preferences.
- Consented home-load baselines during occupied and away periods.
- Actual reward acceptance, churn, complaints, support cost, and retention.
- Battery-specific degradation cost and protected hardware/BMS reserve floors.
- Contractual dispatch revenue, settlement, penalties, and capacity value.
- Effective-dated price catalogs, agreements, eligibility, and reward ledgers.
- Labeled outcomes for unusual-energy-activity alerts.

### Safe initial substitutes

Use seeded personas, vacation windows, early returns, tier adoption, reward
response, and anomaly events for simulation. Backtests may join these synthetic
behaviors to public market, load, weather, and outage history, but every result
must remain `SIMULATED` or `DERIVED` as appropriate. Public aggregate load must
not be used to infer whether a real household is occupied.
