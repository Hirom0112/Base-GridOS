BEGIN;

CREATE TABLE device_command_generations (
    device_id text PRIMARY KEY,
    last_generation bigint NOT NULL CHECK (last_generation >= 0)
);

INSERT INTO device_command_generations (device_id, last_generation)
SELECT device_id, MAX(generation)
FROM command_intents
GROUP BY device_id;

CREATE INDEX command_intents_device_generation_idx
ON command_intents (device_id, generation);

COMMIT;
