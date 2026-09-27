BEGIN;

INSERT INTO audit_journal (occurred_at, actor_id, action, resource_type, resource_id, new_values, correlation_id)
SELECT observed_at, device_id, 'TELEMETRY_RECEIVED', 'dispatch_event', device_id || ':' || sequence, payload, observation_id
FROM telemetry_observations;

DROP TABLE telemetry_observations;

COMMIT;
