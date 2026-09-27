BEGIN;

DROP INDEX IF EXISTS reward_ledger_member_offer_period_idx;
DROP INDEX IF EXISTS reward_ledger_event_member_offer_idx;
ALTER TABLE reward_ledger DROP COLUMN IF EXISTS period_start;

COMMIT;
