BEGIN;

CREATE INDEX IF NOT EXISTS command_intents_event_issued_idx ON command_intents (event_id, issued_at, command_id);
CREATE INDEX IF NOT EXISTS command_acknowledgements_command_received_idx
ON command_acknowledgements (command_id, received_at DESC, acknowledgement_id DESC);

COMMIT;
