BEGIN;

ALTER TABLE reward_ledger DROP CONSTRAINT IF EXISTS reward_ledger_offer_required;

ALTER TABLE travel_flex_windows DROP CONSTRAINT IF EXISTS travel_flex_windows_offer_fk;
ALTER TABLE travel_flex_windows DROP COLUMN IF EXISTS offer_id;

ALTER TABLE resilience_plans DROP CONSTRAINT IF EXISTS resilience_plans_offer_fk;
ALTER TABLE resilience_plans DROP COLUMN IF EXISTS offer_id;

DROP TRIGGER IF EXISTS flexibility_offers_append_only ON flexibility_offers;
ALTER TABLE flexibility_offers DROP CONSTRAINT IF EXISTS flexibility_offers_catalog_fk;
ALTER TABLE flexibility_offers DROP CONSTRAINT IF EXISTS flexibility_offers_terms_check;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS battery_monthly_charge_cents;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS energy_monthly_charge_cents;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS credit_type;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS temporary_reserve_percent;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS consent_version;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS consent_text;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS offer_type;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS member_plan_id;
ALTER TABLE flexibility_offers DROP COLUMN IF EXISTS market;

COMMIT;
