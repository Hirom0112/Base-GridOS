BEGIN;

DROP INDEX command_intents_device_generation_idx;
DROP TABLE device_command_generations;

COMMIT;
