BEGIN;

ALTER TABLE resilience_plans
DROP CONSTRAINT IF EXISTS resilience_plans_catalog_entry_fk;

ALTER TABLE resilience_plans
DROP COLUMN IF EXISTS explanation_shown;

ALTER TABLE resilience_plans
DROP COLUMN IF EXISTS member_plan_id;

ALTER TABLE resilience_plans
DROP COLUMN IF EXISTS catalog_version;

ALTER TABLE pricing_catalog_snapshots
DROP CONSTRAINT IF EXISTS pricing_catalog_snapshots_reserve_floor_check;

DROP TRIGGER IF EXISTS pricing_catalog_snapshots_append_only ON pricing_catalog_snapshots;

ALTER TABLE pricing_catalog_snapshots
DROP CONSTRAINT IF EXISTS pricing_catalog_snapshots_display_name_check;

ALTER TABLE pricing_catalog_snapshots
DROP COLUMN IF EXISTS reserve_floor_percent;

ALTER TABLE pricing_catalog_snapshots
DROP COLUMN IF EXISTS display_name;

COMMIT;
