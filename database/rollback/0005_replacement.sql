BEGIN;

ALTER TABLE plan_versions
DROP COLUMN replacement_snapshot_id;

COMMIT;
