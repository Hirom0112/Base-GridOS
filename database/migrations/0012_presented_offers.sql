BEGIN;

ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS member_plan_id text;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS market text;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS offer_type text;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS consent_text text;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS consent_version text;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS temporary_reserve_percent double precision;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS credit_type text;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS energy_monthly_charge_cents bigint;
ALTER TABLE flexibility_offers ADD COLUMN IF NOT EXISTS battery_monthly_charge_cents bigint;

ALTER TABLE flexibility_offers DROP CONSTRAINT IF EXISTS flexibility_offers_terms_check;
ALTER TABLE flexibility_offers ADD CONSTRAINT flexibility_offers_terms_check CHECK (
    offer_type IS NULL OR
    (member_plan_id <> '' AND market <> '' AND consent_text <> '' AND consent_version <> ''
     AND energy_monthly_charge_cents >= 0 AND battery_monthly_charge_cents >= 0
     AND ((offer_type = 'PLAN' AND temporary_reserve_percent IS NULL AND credit_type IS NULL)
       OR (offer_type = 'TRAVEL_FLEX' AND temporary_reserve_percent BETWEEN 0 AND 100
           AND credit_type IN ('FIXED_DAILY', 'FIXED_EVENT', 'FIXED_ANNUAL'))))
);

ALTER TABLE flexibility_offers DROP CONSTRAINT IF EXISTS flexibility_offers_catalog_fk;
ALTER TABLE flexibility_offers ADD CONSTRAINT flexibility_offers_catalog_fk
FOREIGN KEY (catalog_version, member_plan_id)
REFERENCES pricing_catalog_snapshots (catalog_version, member_plan_id);

DROP TRIGGER IF EXISTS flexibility_offers_append_only ON flexibility_offers;
CREATE TRIGGER flexibility_offers_append_only
BEFORE UPDATE OR DELETE ON flexibility_offers
FOR EACH ROW EXECUTE FUNCTION reject_row_mutation();

ALTER TABLE resilience_plans ADD COLUMN IF NOT EXISTS offer_id text;
ALTER TABLE resilience_plans DROP CONSTRAINT IF EXISTS resilience_plans_offer_fk;
ALTER TABLE resilience_plans ADD CONSTRAINT resilience_plans_offer_fk
FOREIGN KEY (offer_id) REFERENCES flexibility_offers (offer_id);

ALTER TABLE travel_flex_windows ADD COLUMN IF NOT EXISTS offer_id text;
ALTER TABLE travel_flex_windows DROP CONSTRAINT IF EXISTS travel_flex_windows_offer_fk;
ALTER TABLE travel_flex_windows ADD CONSTRAINT travel_flex_windows_offer_fk
FOREIGN KEY (offer_id) REFERENCES flexibility_offers (offer_id);

ALTER TABLE reward_ledger DROP CONSTRAINT IF EXISTS reward_ledger_offer_required;
ALTER TABLE reward_ledger ADD CONSTRAINT reward_ledger_offer_required
CHECK (offer_id IS NOT NULL) NOT VALID;

COMMIT;
