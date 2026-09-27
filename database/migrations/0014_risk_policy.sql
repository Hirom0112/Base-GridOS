BEGIN;

ALTER TABLE reserve_policies ADD COLUMN IF NOT EXISTS provenance jsonb;
UPDATE reserve_policies SET provenance = '{"provenance":"DERIVED"}' WHERE provenance IS NULL;
ALTER TABLE reserve_policies ALTER COLUMN provenance SET NOT NULL;
ALTER TABLE reserve_policies ALTER COLUMN provenance SET DEFAULT '{"provenance":"DERIVED"}';
ALTER TABLE reserve_policies DROP CONSTRAINT IF EXISTS reserve_policies_provenance_check;
ALTER TABLE reserve_policies ADD CONSTRAINT reserve_policies_provenance_check
CHECK (provenance->>'provenance' IN ('CONFIRMED_PUBLIC', 'CONFIRMED_SANDBOX', 'AUTHORIZED_OPERATIONAL', 'DERIVED', 'SIMULATED'));

ALTER TABLE pricing_catalog_snapshots ADD COLUMN IF NOT EXISTS provenance jsonb;
DROP TRIGGER IF EXISTS pricing_catalog_snapshots_append_only ON pricing_catalog_snapshots;
UPDATE pricing_catalog_snapshots SET provenance = '{"provenance":"DERIVED"}' WHERE provenance IS NULL;
CREATE TRIGGER pricing_catalog_snapshots_append_only
BEFORE UPDATE OR DELETE ON pricing_catalog_snapshots
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();
ALTER TABLE pricing_catalog_snapshots ALTER COLUMN provenance SET NOT NULL;
ALTER TABLE pricing_catalog_snapshots ALTER COLUMN provenance SET DEFAULT '{"provenance":"DERIVED"}';
ALTER TABLE pricing_catalog_snapshots DROP CONSTRAINT IF EXISTS pricing_catalog_snapshots_provenance_check;
ALTER TABLE pricing_catalog_snapshots ADD CONSTRAINT pricing_catalog_snapshots_provenance_check
CHECK (provenance->>'provenance' IN ('CONFIRMED_PUBLIC', 'CONFIRMED_SANDBOX', 'AUTHORIZED_OPERATIONAL', 'DERIVED', 'SIMULATED'));

CREATE TABLE IF NOT EXISTS risk_policy (
    version text PRIMARY KEY,
    effective_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    outage_probability_threshold double precision NOT NULL CHECK (outage_probability_threshold BETWEEN 0 AND 1),
    telemetry_freshness_seconds double precision NOT NULL CHECK (telemetry_freshness_seconds > 0),
    gateway_cadence_seconds double precision NOT NULL CHECK (gateway_cadence_seconds > 0),
    weather_floor_percent double precision NOT NULL CHECK (weather_floor_percent BETWEEN 0 AND 100),
    outage_floor_percent double precision NOT NULL CHECK (outage_floor_percent BETWEEN 0 AND 100),
    stale_floor_percent double precision NOT NULL CHECK (stale_floor_percent BETWEEN 0 AND 100),
    alarm_floor_percent double precision NOT NULL CHECK (alarm_floor_percent BETWEEN 0 AND 100),
    communications_floor_percent double precision NOT NULL CHECK (communications_floor_percent BETWEEN 0 AND 100),
    health_floor_percent double precision NOT NULL CHECK (health_floor_percent BETWEEN 0 AND 100),
    weather_zone_ugc jsonb NOT NULL CHECK (jsonb_typeof(weather_zone_ugc) = 'object'),
    provenance jsonb NOT NULL CHECK (provenance->>'provenance' IN ('SIMULATED', 'AUTHORIZED_OPERATIONAL')),
    CHECK (expires_at > effective_at),
    EXCLUDE USING gist (tstzrange(effective_at, expires_at, '[)') WITH &&)
);

DROP TRIGGER IF EXISTS risk_policy_append_only ON risk_policy;
CREATE TRIGGER risk_policy_append_only
BEFORE UPDATE OR DELETE ON risk_policy
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE TABLE IF NOT EXISTS gateway_heartbeats (
    gateway_id text PRIMARY KEY,
    last_published_at timestamptz NOT NULL,
    last_sequence_count integer NOT NULL CHECK (last_sequence_count > 0)
);

CREATE TABLE IF NOT EXISTS gateway_device_sources (
    device_id text PRIMARY KEY,
    gateway_id text NOT NULL REFERENCES gateway_heartbeats(gateway_id),
    observed_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS gateway_device_sources_gateway_idx ON gateway_device_sources (gateway_id);

CREATE TABLE IF NOT EXISTS risk_policy_evaluations (
    site_id text NOT NULL,
    evaluated_at timestamptz NOT NULL,
    member_id text,
    policy_version text NOT NULL REFERENCES risk_policy(version),
    signals jsonb NOT NULL,
    decisions jsonb NOT NULL,
    PRIMARY KEY (site_id, evaluated_at)
);

DROP TRIGGER IF EXISTS risk_policy_evaluations_append_only ON risk_policy_evaluations;
CREATE TRIGGER risk_policy_evaluations_append_only
BEFORE UPDATE OR DELETE ON risk_policy_evaluations
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
