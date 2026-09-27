BEGIN;

DROP TABLE IF EXISTS risk_policy_evaluations;
DROP TABLE IF EXISTS gateway_device_sources;
DROP TABLE IF EXISTS gateway_heartbeats;
DROP TABLE IF EXISTS risk_policy;

ALTER TABLE pricing_catalog_snapshots DROP COLUMN IF EXISTS provenance;
ALTER TABLE reserve_policies DROP COLUMN IF EXISTS provenance;

COMMIT;
