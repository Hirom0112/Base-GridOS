BEGIN;

DROP TRIGGER IF EXISTS plan_versions_append_only ON plan_versions;
DROP TABLE IF EXISTS plan_versions;
DROP TABLE IF EXISTS eligibility_snapshots;
DROP TABLE IF EXISTS input_snapshots;
DROP TABLE IF EXISTS dispatch_events;
DROP TABLE IF EXISTS dispatch_requests;
DROP FUNCTION IF EXISTS reject_row_mutation();

COMMIT;
