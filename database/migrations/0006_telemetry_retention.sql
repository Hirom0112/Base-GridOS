BEGIN;

CREATE TABLE IF NOT EXISTS telemetry_observations (
    observed_at timestamptz NOT NULL,
    device_id text NOT NULL,
    sequence bigint NOT NULL,
    observation_id text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (observed_at, device_id, sequence)
) PARTITION BY RANGE (observed_at);

CREATE TABLE IF NOT EXISTS telemetry_observations_default
PARTITION OF telemetry_observations DEFAULT;

CREATE INDEX IF NOT EXISTS telemetry_observations_latest
ON telemetry_observations (device_id, observed_at DESC, sequence DESC);

CREATE INDEX IF NOT EXISTS telemetry_observations_identity
ON telemetry_observations (device_id, sequence);

INSERT INTO telemetry_observations (observed_at, device_id, sequence, observation_id, payload)
SELECT occurred_at, actor_id, (new_values->>'sequence')::bigint, correlation_id, new_values
FROM audit_journal
WHERE action = 'TELEMETRY_RECEIVED'
AND occurred_at >= (((now() AT TIME ZONE 'UTC')::date - 6)::timestamp AT TIME ZONE 'UTC')
ON CONFLICT DO NOTHING;

DROP TRIGGER IF EXISTS audit_journal_append_only ON audit_journal;
DELETE FROM audit_journal WHERE action = 'TELEMETRY_RECEIVED';
CREATE TRIGGER audit_journal_append_only
BEFORE UPDATE OR DELETE ON audit_journal
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
