package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	storagegen "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/gen"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

type StoredViolation struct {
	Code string `json:"code"`
}

func (store *PostgresEventStore) StoreFrozen(ctx context.Context, eventID string, request *gridosv1.OptimizationRequest, at time.Time) (string, string, error) {
	if request == nil || request.GetPlanVersion() == 0 || request.GetEligibilitySnapshot() == nil {
		return "", "", errors.New("versioned optimization snapshot required")
	}
	inputs, err := protojson.Marshal(request)
	if err != nil {
		return "", "", err
	}
	exclusions, err := exclusionJSON(request.GetEligibilitySnapshot().GetExclusions())
	if err != nil {
		return "", "", err
	}
	inputID, eligibilityID := snapshotIDs(eventID, request.GetPlanVersion())
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO input_snapshots
        (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
        VALUES ($1, $2, $3, $4, '{"source":"SIMULATED"}', $5)
		ON CONFLICT (snapshot_id) DO NOTHING`, inputID, eventID, at, inputs, request.GetCorrelationId())
	if err != nil {
		return "", "", err
	}
	eligibleDeviceIDs := request.GetEligibilitySnapshot().GetEligibleDeviceIds()
	if eligibleDeviceIDs == nil {
		eligibleDeviceIDs = []string{}
	}
	_, err = tx.Exec(ctx, `INSERT INTO eligibility_snapshots
        (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (snapshot_id) DO NOTHING`, eligibilityID, eventID, at, eligibleDeviceIDs, exclusions,
		request.GetReservePolicy().GetPolicyVersion(), request.GetCorrelationId())
	if err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return inputID, eligibilityID, nil
}

func (store *PostgresEventStore) LoadFrozen(ctx context.Context, eventID, inputID, eligibilityID string) (*gridosv1.OptimizationRequest, error) {
	var inputs []byte
	err := store.pool.QueryRow(ctx, `SELECT input.inputs FROM input_snapshots AS input
		JOIN eligibility_snapshots AS eligibility ON eligibility.event_id = input.event_id
		WHERE input.event_id = $1 AND input.snapshot_id = $2 AND eligibility.snapshot_id = $3`, eventID, inputID, eligibilityID).Scan(&inputs)
	if err != nil {
		return nil, err
	}
	request := new(gridosv1.OptimizationRequest)
	if err = protojson.Unmarshal(inputs, request); err != nil {
		return nil, err
	}
	return request, nil
}

func (store *PostgresEventStore) StorePlanned(ctx context.Context, eventID string, request *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan, at time.Time) (*gridosv1.DispatchEvent, error) {
	if request == nil || plan == nil || plan.GetPlanVersion() == 0 {
		return nil, errors.New("optimization request and versioned plan required")
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storagegen.New(tx)
	row, err := queries.LockControlEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if row.State != "REQUESTED" {
		return nil, ErrEventState
	}
	inputs, err := protojson.Marshal(request)
	if err != nil {
		return nil, err
	}
	planJSON, err := protojson.Marshal(plan)
	if err != nil {
		return nil, err
	}
	exclusions, err := exclusionJSON(plan.GetExclusions())
	if err != nil {
		return nil, err
	}
	inputID, eligibilityID := snapshotIDs(eventID, plan.GetPlanVersion())
	eligibleDeviceIDs := request.GetEligibilitySnapshot().GetEligibleDeviceIds()
	if eligibleDeviceIDs == nil {
		eligibleDeviceIDs = []string{}
	}
	_, err = tx.Exec(ctx, `INSERT INTO input_snapshots
        (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ($1, $2, $3, $4, '{"source":"SIMULATED"}', $5)
		ON CONFLICT (snapshot_id) DO NOTHING`, inputID, eventID, at, inputs, request.GetCorrelationId())
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO eligibility_snapshots
        (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (snapshot_id) DO UPDATE SET exclusions = EXCLUDED.exclusions`, eligibilityID, eventID, at,
		eligibleDeviceIDs, exclusions, request.GetReservePolicy().GetPolicyVersion(), request.GetCorrelationId())
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO plan_versions
        (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, eventID, plan.GetPlanVersion(), inputID, eligibilityID,
		planJSON, plan.GetSolverVersion(), plan.GetModelVersion(), request.GetCorrelationId(), at)
	if err != nil {
		return nil, err
	}
	updated, err := transitionLifecycle(ctx, tx, row, "PLANNED", int64(plan.GetPlanVersion()), "service", at)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return eventFromRow(updated, nil), nil
}

func (store *PostgresEventStore) StoreReplacement(ctx context.Context, eventID string, previousVersion uint64, current *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan, key string, at time.Time) error {
	if err := validateReplacement(eventID, previousVersion, current, plan, key); err != nil {
		return err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := storagegen.New(tx).LockControlEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if row.PlanVersion != int64(previousVersion) || row.State != "EXECUTING" {
		return ErrEventState
	}
	var inputID, eligibilityID string
	err = tx.QueryRow(ctx, `SELECT input_snapshot_id, eligibility_snapshot_id FROM plan_versions WHERE event_id = $1 AND version = $2`, eventID, previousVersion).Scan(&inputID, &eligibilityID)
	if err != nil {
		return err
	}
	currentJSON, err := protojson.Marshal(current)
	if err != nil {
		return err
	}
	planJSON, err := protojson.Marshal(plan)
	if err != nil {
		return err
	}
	replacementID := fmt.Sprintf("%s-replacement-%d", eventID, plan.GetPlanVersion())
	_, err = tx.Exec(ctx, `INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ($1, $2, $3, $4, '{"source":"CURRENT_TELEMETRY"}', $5)`, replacementID, eventID, at, currentJSON, current.GetCorrelationId())
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO plan_versions
		(event_id, version, input_snapshot_id, eligibility_snapshot_id, replacement_snapshot_id, plan, solver_version, model_version, correlation_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, eventID, plan.GetPlanVersion(), inputID, eligibilityID, replacementID,
		planJSON, plan.GetSolverVersion(), plan.GetModelVersion(), current.GetCorrelationId(), at)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE dispatch_events SET plan_version = $2, updated_at = $3 WHERE event_id = $1`, eventID, plan.GetPlanVersion(), at)
	if err != nil {
		return err
	}
	values, err := json.Marshal(struct {
		PlanVersion           uint64 `json:"plan_version"`
		ReplacementSnapshotID string `json:"replacement_snapshot_id"`
		IdempotencyKey        string `json:"idempotency_key"`
	}{plan.GetPlanVersion(), replacementID, key})
	if err != nil {
		return err
	}
	if err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: at, ActorID: "decision", Action: "REPLACEMENT_PLANNED", ResourceID: eventID, NewValues: values, CorrelationID: current.GetCorrelationId()}); err != nil {
		return err
	}
	if plan.GetFallbackReason() == "SAFETY_REJECTED" {
		if err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: at, ActorID: "safety", Action: "REPLACEMENT_SAFETY_REJECTED", ResourceID: eventID, NewValues: values, CorrelationID: current.GetCorrelationId()}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func validateReplacement(eventID string, previousVersion uint64, current *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan, key string) error {
	if eventID == "" || key == "" || previousVersion == 0 || current == nil || plan == nil || current.GetEligibilitySnapshot() == nil ||
		current.GetEventId() != eventID || plan.GetEventId() != eventID || current.GetPlanVersion() != previousVersion+1 || plan.GetPlanVersion() != current.GetPlanVersion() {
		return errors.New("complete next-version replacement required")
	}
	return nil
}

func snapshotIDs(eventID string, planVersion uint64) (string, string) {
	return fmt.Sprintf("%s-input-%d", eventID, planVersion), fmt.Sprintf("%s-eligibility-%d", eventID, planVersion)
}

func (store *PostgresEventStore) ValidatePlanned(ctx context.Context, eventID string, planVersion uint64, violations []StoredViolation, at time.Time) (*gridosv1.DispatchEvent, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storagegen.New(tx)
	row, err := queries.LockControlEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if row.State != "PLANNED" || row.PlanVersion != int64(planVersion) {
		return nil, ErrEventState
	}
	if len(violations) > 0 {
		values, marshalErr := json.Marshal(violations)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: at, ActorID: "service", Action: "PLAN_VALIDATION_REJECTED", ResourceID: eventID, NewValues: values, CorrelationID: row.CorrelationID}); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return eventFromRow(row, nil), nil
	}
	updated, err := transitionLifecycle(ctx, tx, row, "VALIDATED", row.PlanVersion, "service", at)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return eventFromRow(updated, nil), nil
}

func (store *PostgresEventStore) LoadPlan(ctx context.Context, eventID string, planVersion uint64) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	var inputs, planJSON []byte
	err := store.pool.QueryRow(ctx, `SELECT input.inputs, plan.plan
        FROM plan_versions AS plan JOIN input_snapshots AS input ON input.snapshot_id = plan.input_snapshot_id
        WHERE plan.event_id = $1 AND plan.version = $2`, eventID, planVersion).Scan(&inputs, &planJSON)
	if err != nil {
		return nil, nil, err
	}
	request := new(gridosv1.OptimizationRequest)
	if err = protojson.Unmarshal(inputs, request); err != nil {
		return nil, nil, err
	}
	plan := new(gridosv1.DispatchPlan)
	if err = protojson.Unmarshal(planJSON, plan); err != nil {
		return nil, nil, err
	}
	return request, plan, nil
}

func (store *PostgresEventStore) Advance(ctx context.Context, eventID, expected, next, actor string, at time.Time) (*gridosv1.DispatchEvent, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storagegen.New(tx)
	row, err := queries.LockControlEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if row.State != expected {
		return nil, ErrEventState
	}
	updated, err := transitionLifecycle(ctx, tx, row, next, row.PlanVersion, actor, at)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return eventFromRow(updated, nil), nil
}

func (store *PostgresEventStore) Violations(ctx context.Context, eventID string) ([]StoredViolation, error) {
	var contents []byte
	err := store.pool.QueryRow(ctx, `SELECT new_values FROM audit_journal
        WHERE resource_id = $1 AND action = 'PLAN_VALIDATION_REJECTED'
        ORDER BY sequence DESC LIMIT 1`, eventID).Scan(&contents)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var violations []StoredViolation
	err = json.Unmarshal(contents, &violations)
	return violations, err
}

func transitionLifecycle(ctx context.Context, tx pgx.Tx, row storagegen.DispatchEvent, next string, planVersion int64, actor string, at time.Time) (storagegen.DispatchEvent, error) {
	var updated storagegen.DispatchEvent
	err := tx.QueryRow(ctx, `UPDATE dispatch_events SET state = $1, plan_version = $2, updated_at = $3
        WHERE event_id = $4 AND state = $5 RETURNING event_id, request_id, state, plan_version, correlation_id, created_at, updated_at`,
		next, planVersion, at, row.EventID, row.State).Scan(&updated.EventID, &updated.RequestID, &updated.State, &updated.PlanVersion, &updated.CorrelationID, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return updated, err
	}
	previous, _ := json.Marshal(eventStateValue{State: row.State})
	current, _ := json.Marshal(eventStateValue{State: next})
	err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: at, ActorID: actor, Action: "EVENT_STATE_TRANSITIONED", ResourceID: row.EventID, PreviousValues: previous, NewValues: current, CorrelationID: row.CorrelationID})
	return updated, err
}

func exclusionJSON(exclusions []*gridosv1.DeviceExclusion) ([]byte, error) {
	values := make([]struct {
		DeviceID string `json:"device_id"`
		Reason   string `json:"reason"`
	}, 0, len(exclusions))
	for _, exclusion := range exclusions {
		values = append(values, struct {
			DeviceID string `json:"device_id"`
			Reason   string `json:"reason"`
		}{DeviceID: exclusion.GetDeviceId(), Reason: exclusion.GetReason().String()})
	}
	return json.Marshal(values)
}
