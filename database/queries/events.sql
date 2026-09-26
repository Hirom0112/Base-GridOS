-- name: TransitionEventState :one
UPDATE dispatch_events
SET state = sqlc.arg(next_state),
    updated_at = sqlc.arg(transitioned_at)
WHERE event_id = sqlc.arg(event_id)
  AND state = sqlc.arg(expected_state)
RETURNING *;
