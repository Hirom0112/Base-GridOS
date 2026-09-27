BEGIN;

ALTER TABLE travel_flex_windows
DROP CONSTRAINT IF EXISTS travel_flex_windows_cancelled_at_check;

DROP INDEX IF EXISTS travel_flex_windows_end_key_unique;

ALTER TABLE travel_flex_windows
DROP COLUMN IF EXISTS end_correlation_id;

ALTER TABLE travel_flex_windows
DROP COLUMN IF EXISTS end_idempotency_key;

ALTER TABLE travel_flex_windows
DROP COLUMN IF EXISTS cancelled_at;

COMMIT;
