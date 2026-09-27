BEGIN;

CREATE TABLE IF NOT EXISTS event_reports (
    event_id text NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    report bytea NOT NULL,
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    produced_at timestamptz NOT NULL,
    PRIMARY KEY (event_id, version),
    FOREIGN KEY (event_id, version) REFERENCES plan_versions(event_id, version)
);

DROP TRIGGER IF EXISTS event_reports_append_only ON event_reports;
CREATE TRIGGER event_reports_append_only
BEFORE UPDATE OR DELETE ON event_reports
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

COMMIT;
