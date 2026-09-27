BEGIN;

ALTER TABLE reward_ledger ADD COLUMN IF NOT EXISTS period_start timestamptz;

CREATE UNIQUE INDEX IF NOT EXISTS reward_ledger_event_member_offer_idx
ON reward_ledger (event_id, member_id, offer_id)
WHERE event_id IS NOT NULL AND offer_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS reward_ledger_member_offer_period_idx
ON reward_ledger (member_id, offer_id, period_start)
WHERE event_id IS NULL AND offer_id IS NOT NULL AND period_start IS NOT NULL;

COMMIT;
