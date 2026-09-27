BEGIN;

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

COMMIT;
