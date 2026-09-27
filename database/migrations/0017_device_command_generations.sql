BEGIN;

CREATE TABLE IF NOT EXISTS device_command_generations (
    device_id text PRIMARY KEY,
    last_generation bigint NOT NULL CHECK (last_generation >= 0)
);

INSERT INTO device_command_generations (device_id, last_generation)
SELECT device_id, MAX(generation)
FROM command_intents
GROUP BY device_id
ON CONFLICT (device_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS command_intents_device_generation_idx
ON command_intents (device_id, generation);

COMMIT;
