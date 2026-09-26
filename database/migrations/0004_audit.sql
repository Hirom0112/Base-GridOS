BEGIN;

CREATE TABLE IF NOT EXISTS operator_approvals (
    approval_id text PRIMARY KEY,
    event_id text NOT NULL,
    plan_version bigint NOT NULL,
    decision text NOT NULL CHECK (decision IN ('APPROVED', 'REJECTED')),
    decided_by text NOT NULL,
    decided_at timestamptz NOT NULL,
    rationale text NOT NULL,
    correlation_id text NOT NULL,
    FOREIGN KEY (event_id, plan_version) REFERENCES plan_versions(event_id, version)
);

CREATE TABLE IF NOT EXISTS emergency_stops (
    emergency_stop_id text PRIMARY KEY,
    idempotency_key text NOT NULL UNIQUE,
    event_id text NOT NULL REFERENCES dispatch_events(event_id),
    requested_by text NOT NULL,
    reason text NOT NULL,
    requested_at timestamptz NOT NULL,
    correlation_id text NOT NULL
);

CREATE TABLE IF NOT EXISTS verification_summaries (
    verification_id text PRIMARY KEY,
    event_id text NOT NULL REFERENCES dispatch_events(event_id),
    interval_begin_time timestamptz NOT NULL,
    interval_end_time timestamptz NOT NULL,
    requested_kw double precision NOT NULL,
    commanded_kw double precision NOT NULL,
    delivered_kw double precision NOT NULL,
    tracking_error_kw double precision NOT NULL,
    confidence double precision NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    baseline_method text NOT NULL,
    measurement_boundary text NOT NULL,
    correlation_id text NOT NULL,
    CHECK (interval_end_time > interval_begin_time)
);

CREATE TABLE IF NOT EXISTS audit_journal (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_id text NOT NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text NOT NULL,
    previous_values jsonb,
    new_values jsonb,
    correlation_id text NOT NULL
);

DROP TRIGGER IF EXISTS operator_approvals_append_only ON operator_approvals;
CREATE TRIGGER operator_approvals_append_only
BEFORE UPDATE OR DELETE ON operator_approvals
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

DROP TRIGGER IF EXISTS emergency_stops_append_only ON emergency_stops;
CREATE TRIGGER emergency_stops_append_only
BEFORE UPDATE OR DELETE ON emergency_stops
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

DROP TRIGGER IF EXISTS audit_journal_append_only ON audit_journal;
CREATE TRIGGER audit_journal_append_only
BEFORE UPDATE OR DELETE ON audit_journal
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
