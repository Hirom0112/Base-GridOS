BEGIN;

ALTER TABLE plan_versions
ADD COLUMN IF NOT EXISTS replacement_snapshot_id text REFERENCES input_snapshots(snapshot_id);

COMMIT;
