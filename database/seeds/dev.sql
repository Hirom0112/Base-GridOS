BEGIN;

CREATE TABLE IF NOT EXISTS sites (
    site_id text PRIMARY KEY,
    weather_zone text NOT NULL,
    load_zone text NOT NULL,
    h3_cell text NOT NULL,
    has_solar boolean NOT NULL,
    has_automatic_backup boolean NOT NULL,
    provenance text NOT NULL CHECK (provenance IN ('CONFIRMED_PUBLIC', 'CONFIRMED_SANDBOX', 'AUTHORIZED_OPERATIONAL', 'DERIVED', 'SIMULATED')),
    simulation_seed bigint,
    correlation_id text NOT NULL
);

CREATE TEMPORARY TABLE fleet_seed (
    record jsonb NOT NULL
) ON COMMIT DROP;

\copy fleet_seed (record) FROM 'testdata/fleets/texas-50.jsonl'

INSERT INTO sites (
    site_id,
    weather_zone,
    load_zone,
    h3_cell,
    has_solar,
    has_automatic_backup,
    provenance,
    simulation_seed,
    correlation_id
)
SELECT
    record->>'site_id',
    record->>'weather_zone',
    record->>'load_zone',
    record->>'h3_cell',
    (record->>'has_solar')::boolean,
    (record->>'has_automatic_backup')::boolean,
    record->>'provenance',
    (record->>'simulation_seed')::bigint,
    'seed:texas-50'
FROM fleet_seed
ON CONFLICT (site_id) DO UPDATE
SET weather_zone = EXCLUDED.weather_zone,
    load_zone = EXCLUDED.load_zone,
    h3_cell = EXCLUDED.h3_cell,
    has_solar = EXCLUDED.has_solar,
    has_automatic_backup = EXCLUDED.has_automatic_backup,
    provenance = EXCLUDED.provenance,
    simulation_seed = EXCLUDED.simulation_seed,
    correlation_id = EXCLUDED.correlation_id;

INSERT INTO risk_policy (
    version, effective_at, expires_at, outage_probability_threshold,
    telemetry_freshness_seconds, gateway_cadence_seconds, weather_floor_percent,
    outage_floor_percent, stale_floor_percent, alarm_floor_percent,
    communications_floor_percent, health_floor_percent, provenance
) VALUES (
    'risk-policy-sim-1', '2020-01-01T00:00:00Z', '2100-01-01T00:00:00Z', 0.01,
    30, 15, 60, 60, 40, 100, 40, 100, '{"provenance":"SIMULATED"}'
) ON CONFLICT (version) DO NOTHING;

COMMIT;
