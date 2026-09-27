BEGIN;

DROP TRIGGER IF EXISTS reward_ledger_append_only ON reward_ledger;
DROP TABLE IF EXISTS reward_ledger;
DROP TABLE IF EXISTS flexibility_offers;
DROP TABLE IF EXISTS plan_add_ons;
DROP TABLE IF EXISTS pricing_catalog_snapshots;
DROP TABLE IF EXISTS reserve_overrides;
DROP TABLE IF EXISTS travel_flex_windows;
DROP TABLE IF EXISTS resilience_plans;
DROP TABLE IF EXISTS reserve_policies;

COMMIT;
