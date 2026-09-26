BEGIN;

ALTER TABLE plan_versions
ADD COLUMN replacement_snapshot_id text REFERENCES input_snapshots(snapshot_id);

COMMIT;
