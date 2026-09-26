-- name: InsertDispatchRequest :exec
INSERT INTO dispatch_requests (
    request_id, event_type, begin_time, end_time, target_kw,
    measurement_boundary, load_zones, correlation_id, created_at
) VALUES (
    sqlc.arg(request_id), sqlc.arg(event_type), sqlc.arg(begin_time),
    sqlc.arg(end_time), sqlc.arg(target_kw), sqlc.arg(measurement_boundary),
    sqlc.arg(load_zones), sqlc.arg(correlation_id), sqlc.arg(created_at)
);

-- name: InsertDispatchEvent :one
INSERT INTO dispatch_events (
    event_id, request_id, state, plan_version, correlation_id, created_at, updated_at
) VALUES (
    sqlc.arg(event_id), sqlc.arg(request_id), sqlc.arg(state),
    sqlc.arg(plan_version), sqlc.arg(correlation_id), sqlc.arg(created_at),
    sqlc.arg(updated_at)
)
RETURNING *;

-- name: GetControlEvent :one
SELECT * FROM dispatch_events WHERE event_id = sqlc.arg(event_id);

-- name: LockControlEvent :one
SELECT * FROM dispatch_events WHERE event_id = sqlc.arg(event_id) FOR UPDATE;

-- name: FindAuditResourceByIdempotency :one
SELECT resource_id
FROM audit_journal
WHERE action = sqlc.arg(action)
  AND new_values ->> 'idempotency_key' = sqlc.arg(idempotency_key)::text
ORDER BY sequence DESC
LIMIT 1;

-- name: LatestEventLaunch :one
SELECT new_values
FROM audit_journal
WHERE resource_type = 'dispatch_event'
  AND resource_id = sqlc.arg(event_id)
  AND action = 'EVENT_LAUNCHED'
ORDER BY sequence DESC
LIMIT 1;

-- name: LatestEligibilityExclusions :one
SELECT exclusions
FROM eligibility_snapshots
WHERE event_id = sqlc.arg(event_id)
ORDER BY captured_at DESC
LIMIT 1;

-- name: InsertOperatorApproval :exec
INSERT INTO operator_approvals (
    approval_id, event_id, plan_version, decision, decided_by,
    decided_at, rationale, correlation_id
) VALUES (
    sqlc.arg(approval_id), sqlc.arg(event_id), sqlc.arg(plan_version),
    'APPROVED', sqlc.arg(decided_by), sqlc.arg(decided_at), '',
    sqlc.arg(correlation_id)
);
