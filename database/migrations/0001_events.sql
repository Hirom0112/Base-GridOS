BEGIN;

CREATE TABLE IF NOT EXISTS dispatch_requests (
    request_id text PRIMARY KEY,
    event_type text NOT NULL,
    begin_time timestamptz NOT NULL,
    end_time timestamptz NOT NULL,
    target_kw double precision NOT NULL CHECK (target_kw >= 0),
    measurement_boundary text NOT NULL,
    load_zones text[] NOT NULL,
    correlation_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (end_time > begin_time)
);

CREATE TABLE IF NOT EXISTS dispatch_events (
    event_id text PRIMARY KEY,
    request_id text NOT NULL REFERENCES dispatch_requests(request_id),
    state text NOT NULL CHECK (state IN (
        'REQUESTED',
        'PLANNED',
        'VALIDATED',
        'APPROVED',
        'COMMANDS_PERSISTED',
        'SENT',
        'ACKNOWLEDGED_OR_UNCERTAIN',
        'EXECUTING',
        'VERIFIED',
        'RECONCILED',
        'REPORTED'
    )),
    plan_version bigint NOT NULL DEFAULT 0 CHECK (plan_version >= 0),
    correlation_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS input_snapshots (
    snapshot_id text PRIMARY KEY,
    event_id text NOT NULL REFERENCES dispatch_events(event_id),
    captured_at timestamptz NOT NULL,
    inputs jsonb NOT NULL,
    provenance jsonb NOT NULL,
    correlation_id text NOT NULL
);

CREATE TABLE IF NOT EXISTS eligibility_snapshots (
    snapshot_id text PRIMARY KEY,
    event_id text NOT NULL REFERENCES dispatch_events(event_id),
    captured_at timestamptz NOT NULL,
    eligible_device_ids text[] NOT NULL,
    exclusions jsonb NOT NULL,
    policy_version text NOT NULL,
    correlation_id text NOT NULL
);

CREATE TABLE IF NOT EXISTS plan_versions (
    event_id text NOT NULL REFERENCES dispatch_events(event_id),
    version bigint NOT NULL CHECK (version > 0),
    input_snapshot_id text NOT NULL REFERENCES input_snapshots(snapshot_id),
    eligibility_snapshot_id text NOT NULL REFERENCES eligibility_snapshots(snapshot_id),
    plan jsonb NOT NULL,
    solver_version text NOT NULL,
    model_version text NOT NULL,
    correlation_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, version)
);

CREATE OR REPLACE FUNCTION reject_row_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    RAISE EXCEPTION '% rows are append-only', TG_TABLE_NAME;
END;
$function$;

DROP TRIGGER IF EXISTS plan_versions_append_only ON plan_versions;
CREATE TRIGGER plan_versions_append_only
BEFORE UPDATE OR DELETE ON plan_versions
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
