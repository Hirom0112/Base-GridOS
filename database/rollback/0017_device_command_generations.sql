BEGIN;

DROP INDEX IF EXISTS command_intents_device_generation_idx;
DROP TABLE IF EXISTS device_command_generations;

COMMIT;
