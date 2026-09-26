BEGIN;

CREATE TABLE IF NOT EXISTS command_intents (
    command_id text PRIMARY KEY,
    idempotency_key text NOT NULL UNIQUE,
    device_id text NOT NULL,
    event_id text NOT NULL REFERENCES dispatch_events(event_id),
    plan_version bigint NOT NULL CHECK (plan_version > 0),
    generation bigint NOT NULL CHECK (generation >= 0),
    setpoint_kw double precision NOT NULL,
    issued_at timestamptz NOT NULL,
    effective_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    policy_version text NOT NULL,
    correlation_id text NOT NULL,
    UNIQUE (command_id),
    CHECK (expires_at > effective_at),
    FOREIGN KEY (event_id, plan_version) REFERENCES plan_versions(event_id, version)
);

CREATE TABLE IF NOT EXISTS command_outbox (
    command_id text PRIMARY KEY REFERENCES command_intents(command_id),
    state text NOT NULL CHECK (state IN ('PENDING', 'PUBLISHING', 'PUBLISHED')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    correlation_id text NOT NULL,
    CHECK ((state = 'PUBLISHED') = (published_at IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS command_states (
    command_id text NOT NULL REFERENCES command_intents(command_id),
    state text NOT NULL CHECK (state IN (
        'PERSISTED',
        'SENT',
        'ACKNOWLEDGED',
        'UNCERTAIN',
        'EXECUTING',
        'COMPLETED',
        'REJECTED',
        'CANCELLED',
        'EXPIRED'
    )),
    recorded_at timestamptz NOT NULL,
    correlation_id text NOT NULL,
    PRIMARY KEY (command_id, recorded_at)
);

CREATE TABLE IF NOT EXISTS command_acknowledgements (
    acknowledgement_id text PRIMARY KEY,
    command_id text NOT NULL REFERENCES command_intents(command_id),
    idempotency_key text NOT NULL UNIQUE,
    receipt_status text NOT NULL CHECK (receipt_status IN ('ACCEPTED', 'REJECTED')),
    received_at timestamptz NOT NULL,
    gateway_id text NOT NULL,
    rejection_reason text NOT NULL DEFAULT '',
    correlation_id text NOT NULL
);

CREATE TABLE IF NOT EXISTS uncertainty_intervals (
    device_id text NOT NULL,
    interval_begin_time timestamptz NOT NULL,
    interval_end_time timestamptz NOT NULL,
    signed_feasible_power_lower_kw double precision NOT NULL,
    signed_feasible_power_upper_kw double precision NOT NULL,
    last_confirmed_command_id text NOT NULL,
    last_confirmed_setpoint_kw double precision NOT NULL,
    possibly_accepted_command_id text NOT NULL,
    possibly_accepted_setpoint_kw double precision NOT NULL,
    possibly_accepted_effective_at timestamptz NOT NULL,
    possibly_accepted_expires_at timestamptz NOT NULL,
    max_ramp_kw_per_second double precision NOT NULL CHECK (max_ramp_kw_per_second >= 0),
    fresh_telemetry_power_kw double precision NOT NULL,
    fresh_telemetry_observed_at timestamptz NOT NULL,
    derived_at timestamptz NOT NULL,
    correlation_id text NOT NULL,
    PRIMARY KEY (device_id, interval_begin_time, interval_end_time),
    CHECK (interval_end_time > interval_begin_time),
    CHECK (signed_feasible_power_lower_kw <= signed_feasible_power_upper_kw)
);

DROP TRIGGER IF EXISTS command_intents_append_only ON command_intents;
CREATE TRIGGER command_intents_append_only
BEFORE UPDATE OR DELETE ON command_intents
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

DROP TRIGGER IF EXISTS command_states_append_only ON command_states;
CREATE TRIGGER command_states_append_only
BEFORE UPDATE OR DELETE ON command_states
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
