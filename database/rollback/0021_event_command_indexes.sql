BEGIN;

DROP INDEX IF EXISTS command_acknowledgements_command_received_idx;
DROP INDEX IF EXISTS command_intents_event_issued_idx;

COMMIT;
