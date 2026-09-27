BEGIN;

ALTER TABLE reserve_overrides
ADD COLUMN IF NOT EXISTS evidence_id text;

ALTER TABLE reserve_overrides
DROP CONSTRAINT IF EXISTS reserve_overrides_reason_check;

ALTER TABLE reserve_overrides
ADD CONSTRAINT reserve_overrides_reason_check
CHECK (reason IN ('WEATHER', 'OUTAGE_RISK', 'HEALTH', 'STALE_TELEMETRY', 'ALARM', 'COMMUNICATIONS', 'EARLY_RETURN'));

COMMIT;
