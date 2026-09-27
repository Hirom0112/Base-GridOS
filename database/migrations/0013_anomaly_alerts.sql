BEGIN;

CREATE TABLE IF NOT EXISTS member_anomaly_preferences (
    preference_id text PRIMARY KEY,
    member_id text NOT NULL,
    opted_in boolean NOT NULL,
    consent_text text NOT NULL,
    consent_version text NOT NULL,
    baseline_upper_kw double precision NOT NULL CHECK (baseline_upper_kw >= 0 AND baseline_upper_kw < 1000000),
    baseline_begin timestamptz NOT NULL,
    baseline_end timestamptz NOT NULL,
    effective_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    correlation_id text NOT NULL,
    CHECK (baseline_end > baseline_begin),
    CHECK (expires_at > effective_at)
);

CREATE INDEX IF NOT EXISTS member_anomaly_preferences_current_idx
ON member_anomaly_preferences (member_id, effective_at DESC);

DROP TRIGGER IF EXISTS member_anomaly_preferences_append_only ON member_anomaly_preferences;
CREATE TRIGGER member_anomaly_preferences_append_only
BEFORE UPDATE OR DELETE ON member_anomaly_preferences
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE TABLE IF NOT EXISTS member_away_periods (
    away_period_id text PRIMARY KEY,
    member_id text NOT NULL,
    start_time timestamptz NOT NULL,
    end_time timestamptz NOT NULL,
    consent_version text NOT NULL,
    correlation_id text NOT NULL,
    ended_at timestamptz,
    end_idempotency_key text UNIQUE,
    end_correlation_id text,
    CHECK (end_time > start_time),
    CHECK (ended_at IS NULL OR (ended_at >= start_time AND ended_at < end_time))
);

CREATE INDEX IF NOT EXISTS member_away_periods_current_idx
ON member_away_periods (member_id, start_time DESC);

CREATE TABLE IF NOT EXISTS member_alerts (
    alert_id text PRIMARY KEY,
    member_id text NOT NULL,
    kind text NOT NULL CHECK (kind = 'ENERGY_ANOMALY_SIGNAL'),
    message text NOT NULL CHECK (message = 'energy anomaly signal'),
    preference_id text NOT NULL REFERENCES member_anomaly_preferences(preference_id),
    evidence jsonb NOT NULL,
    observed_at timestamptz NOT NULL,
    correlation_id text NOT NULL
);

DROP TRIGGER IF EXISTS member_alerts_append_only ON member_alerts;
CREATE TRIGGER member_alerts_append_only
BEFORE UPDATE OR DELETE ON member_alerts
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

CREATE TABLE IF NOT EXISTS member_alert_deliveries (
    delivery_id text PRIMARY KEY,
    alert_id text NOT NULL REFERENCES member_alerts(alert_id),
    channel text NOT NULL,
    attempted_at timestamptz NOT NULL,
    outcome text NOT NULL,
    correlation_id text NOT NULL
);

DROP TRIGGER IF EXISTS member_alert_deliveries_append_only ON member_alert_deliveries;
CREATE TRIGGER member_alert_deliveries_append_only
BEFORE UPDATE OR DELETE ON member_alert_deliveries
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
