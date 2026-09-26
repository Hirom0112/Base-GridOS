-- name: InsertCommandIntentAndOutbox :one
WITH inserted_intent AS (
    INSERT INTO command_intents (
        command_id,
        idempotency_key,
        device_id,
        event_id,
        plan_version,
        generation,
        setpoint_kw,
        issued_at,
        effective_at,
        expires_at,
        policy_version,
        correlation_id
    ) VALUES (
        sqlc.arg(command_id),
        sqlc.arg(idempotency_key),
        sqlc.arg(device_id),
        sqlc.arg(event_id),
        sqlc.arg(plan_version),
        sqlc.arg(generation),
        sqlc.arg(setpoint_kw),
        sqlc.arg(issued_at),
        sqlc.arg(effective_at),
        sqlc.arg(expires_at),
        sqlc.arg(policy_version),
        sqlc.arg(correlation_id)
    )
    RETURNING *
), inserted_outbox AS (
    INSERT INTO command_outbox (
        command_id,
        state,
        attempts,
        next_attempt_at,
        correlation_id
    )
    SELECT
        command_id,
        'PENDING',
        0,
        issued_at,
        correlation_id
    FROM inserted_intent
    RETURNING command_id
)
SELECT inserted_intent.*
FROM inserted_intent
JOIN inserted_outbox USING (command_id);

-- name: ClaimCommandOutboxBatch :many
WITH claimed AS (
    SELECT command_id
    FROM command_outbox
    WHERE state = 'PENDING'
      AND next_attempt_at <= sqlc.arg(available_at)
    ORDER BY next_attempt_at, command_id
    FOR UPDATE SKIP LOCKED
    LIMIT sqlc.arg(batch_size)
)
UPDATE command_outbox AS command_outbox
SET state = 'PUBLISHING',
    attempts = command_outbox.attempts + 1
FROM claimed
WHERE command_outbox.command_id = claimed.command_id
RETURNING command_outbox.*;

-- name: UpsertCommandAcknowledgement :one
INSERT INTO command_acknowledgements (
    acknowledgement_id,
    command_id,
    idempotency_key,
    receipt_status,
    received_at,
    gateway_id,
    rejection_reason,
    correlation_id
) VALUES (
    sqlc.arg(acknowledgement_id),
    sqlc.arg(command_id),
    sqlc.arg(idempotency_key),
    sqlc.arg(receipt_status),
    sqlc.arg(received_at),
    sqlc.arg(gateway_id),
    sqlc.arg(rejection_reason),
    sqlc.arg(correlation_id)
)
ON CONFLICT (acknowledgement_id) DO UPDATE
SET receipt_status = EXCLUDED.receipt_status,
    received_at = EXCLUDED.received_at,
    gateway_id = EXCLUDED.gateway_id,
    rejection_reason = EXCLUDED.rejection_reason,
    correlation_id = EXCLUDED.correlation_id
RETURNING *;
