BEGIN;

CREATE TABLE step_up_assertions (
    nonce text PRIMARY KEY,
    subject text NOT NULL CHECK (subject <> ''),
    action text NOT NULL CHECK (action IN ('APPROVE_EVENT', 'EMERGENCY_STOP')),
    event_id text NOT NULL,
    plan_version bigint NOT NULL CHECK (plan_version >= 0),
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz NOT NULL,
    CHECK (expires_at > issued_at AND expires_at <= issued_at + interval '5 minutes')
);

CREATE TRIGGER step_up_assertions_append_only
BEFORE UPDATE OR DELETE ON step_up_assertions
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
