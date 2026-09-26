-- name: AppendAudit :one
INSERT INTO audit_journal (
    occurred_at,
    actor_id,
    action,
    resource_type,
    resource_id,
    previous_values,
    new_values,
    correlation_id
) VALUES (
    sqlc.arg(occurred_at),
    sqlc.arg(actor_id),
    sqlc.arg(action),
    sqlc.arg(resource_type),
    sqlc.arg(resource_id),
    sqlc.narg(previous_values),
    sqlc.narg(new_values),
    sqlc.arg(correlation_id)
)
RETURNING *;
