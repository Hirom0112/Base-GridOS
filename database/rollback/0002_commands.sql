BEGIN;

DROP TRIGGER IF EXISTS command_states_append_only ON command_states;
DROP TRIGGER IF EXISTS command_intents_append_only ON command_intents;
DROP TABLE IF EXISTS uncertainty_intervals;
DROP TABLE IF EXISTS command_acknowledgements;
DROP TABLE IF EXISTS command_states;
DROP TABLE IF EXISTS command_outbox;
DROP TABLE IF EXISTS command_intents;

COMMIT;
