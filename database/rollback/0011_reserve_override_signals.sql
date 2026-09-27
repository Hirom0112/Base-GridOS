BEGIN;

ALTER TABLE reserve_overrides
DROP CONSTRAINT IF EXISTS reserve_overrides_reason_check;

ALTER TABLE reserve_overrides
ADD CONSTRAINT reserve_overrides_reason_check
CHECK (reason IN ('WEATHER', 'OUTAGE_RISK', 'HEALTH', 'STALE_TELEMETRY', 'ALARM', 'EARLY_RETURN'));

ALTER TABLE reserve_overrides
DROP COLUMN IF EXISTS evidence_id;

COMMIT;
