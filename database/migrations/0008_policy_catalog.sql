BEGIN;

ALTER TABLE pricing_catalog_snapshots
ADD COLUMN IF NOT EXISTS display_name text;

ALTER TABLE pricing_catalog_snapshots
ADD COLUMN IF NOT EXISTS reserve_floor_percent double precision;

ALTER TABLE pricing_catalog_snapshots
DROP CONSTRAINT IF EXISTS pricing_catalog_snapshots_display_name_check;

ALTER TABLE pricing_catalog_snapshots
ADD CONSTRAINT pricing_catalog_snapshots_display_name_check
CHECK (display_name IS NULL OR length(trim(display_name)) > 0);

ALTER TABLE pricing_catalog_snapshots
DROP CONSTRAINT IF EXISTS pricing_catalog_snapshots_reserve_floor_check;

ALTER TABLE pricing_catalog_snapshots
ADD CONSTRAINT pricing_catalog_snapshots_reserve_floor_check
CHECK (reserve_floor_percent IS NULL OR reserve_floor_percent BETWEEN 0 AND 100);

DROP TRIGGER IF EXISTS pricing_catalog_snapshots_append_only ON pricing_catalog_snapshots;
CREATE TRIGGER pricing_catalog_snapshots_append_only
BEFORE UPDATE OR DELETE ON pricing_catalog_snapshots
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

ALTER TABLE resilience_plans
ADD COLUMN IF NOT EXISTS catalog_version text;

ALTER TABLE resilience_plans
ADD COLUMN IF NOT EXISTS member_plan_id text;

ALTER TABLE resilience_plans
ADD COLUMN IF NOT EXISTS explanation_shown text;

ALTER TABLE resilience_plans
DROP CONSTRAINT IF EXISTS resilience_plans_catalog_entry_fk;

ALTER TABLE resilience_plans
ADD CONSTRAINT resilience_plans_catalog_entry_fk
FOREIGN KEY (catalog_version, member_plan_id)
REFERENCES pricing_catalog_snapshots(catalog_version, member_plan_id);

COMMIT;
