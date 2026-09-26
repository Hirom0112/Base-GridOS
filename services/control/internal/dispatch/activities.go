package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	reporting "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/protobuf/proto"
)

type Activities struct {
	Dispatcher *controlapi.Dispatcher
	Events     controlapi.LifecycleStore
	Pool       *pgxpool.Pool
	Reports    reporting.Source
	Now        func() time.Time
}

type FrozenEvent struct {
	Input                 Input
	InputSnapshotID       string
	EligibilitySnapshotID string
	SnapshotDigest        [32]byte
}

func (activities *Activities) FreezeInputs(ctx context.Context, input Input) (FrozenEvent, error) {
	if input.Request == nil {
		return FrozenEvent{}, errors.New("event request required")
	}
	if activities.Dispatcher == nil || activities.Dispatcher.Optimizer == nil {
		return FrozenEvent{}, errors.New("forecast service required")
	}
	snapshot, err := activities.Dispatcher.Freeze(ctx, input.EventID, input.Request)
	if err != nil {
		return FrozenEvent{}, err
	}
	if snapshot.Optimization == nil || snapshot.Optimization.GetBudget() == nil {
		return FrozenEvent{}, errors.New("versioned forecast request with budget required")
	}
	budget := snapshot.Optimization.GetBudget().AsDuration()
	if budget <= 0 {
		return FrozenEvent{}, errors.New("positive forecast budget required")
	}
	forecastCtx, cancel := context.WithTimeout(ctx, budget+time.Second)
	forecast, err := activities.Dispatcher.Optimizer.Forecast(forecastCtx, &gridosv1.ForecastRequest{Request: snapshot.Optimization})
	cancel()
	if errors.Is(err, context.DeadlineExceeded) || connect.CodeOf(err) == connect.CodeDeadlineExceeded {
		forecast = &gridosv1.ForecastResponse{UnavailableSources: []string{"forecast_transport_timeout"}}
		if err = activities.recordPlanningDecision(ctx, snapshot.Optimization, "FORECAST_TIMEOUT", "TRANSPORT_TIMEOUT"); err != nil {
			return FrozenEvent{}, err
		}
	} else if err != nil {
		return FrozenEvent{}, err
	}
	if forecast == nil {
		return FrozenEvent{}, errors.New("forecast response required")
	}
	snapshot.Optimization.Forecast = forecast
	inputID, eligibilityID, err := storage.NewPostgresEventStore(activities.Pool).StoreFrozen(ctx, input.EventID, snapshot.Optimization, activities.Now())
	if err != nil {
		return FrozenEvent{}, err
	}
	stored, err := storage.NewPostgresEventStore(activities.Pool).LoadFrozen(ctx, input.EventID, inputID, eligibilityID)
	if err != nil {
		return FrozenEvent{}, err
	}
	if !proto.Equal(stored, snapshot.Optimization) {
		return FrozenEvent{}, errors.New("frozen snapshot changed on retry")
	}
	digest, err := snapshotDigest(stored)
	return FrozenEvent{Input: input, InputSnapshotID: inputID, EligibilitySnapshotID: eligibilityID, SnapshotDigest: digest}, err
}

func (activities *Activities) RequestPlan(ctx context.Context, frozen FrozenEvent) (FrozenEvent, error) {
	request, err := storage.NewPostgresEventStore(activities.Pool).LoadFrozen(ctx, frozen.Input.EventID, frozen.InputSnapshotID, frozen.EligibilitySnapshotID)
	if err != nil {
		return frozen, err
	}
	if err = frozen.verifySnapshot(request); err != nil {
		return frozen, err
	}
	if request.GetBudget() == nil {
		return frozen, errors.New("optimization budget required")
	}
	budget := request.GetBudget().AsDuration()
	if budget <= 0 {
		return frozen, errors.New("positive optimization budget required")
	}
	planCtx, cancel := context.WithTimeout(ctx, budget+time.Second)
	defer cancel()
	plan, err := activities.Dispatcher.RequestPlan(planCtx, controlapi.FrozenSnapshot{Optimization: request})
	if err != nil {
		return frozen, err
	}
	if plan.GetFallbackUsed() {
		if err = activities.recordPlanningDecision(ctx, request, "PLAN_FALLBACK_SELECTED", plan.GetFallbackReason()); err != nil {
			return frozen, err
		}
	}
	frozen.Input.PlanVersion = plan.GetPlanVersion()
	frozen.Input.ApprovalDigest, err = approvalDigest(request, plan)
	return frozen, err
}

func (activities *Activities) ValidatePlan(ctx context.Context, frozen FrozenEvent) error {
	request, err := storage.NewPostgresEventStore(activities.Pool).LoadFrozen(ctx, frozen.Input.EventID, frozen.InputSnapshotID, frozen.EligibilitySnapshotID)
	if err != nil {
		return err
	}
	if err = frozen.verifySnapshot(request); err != nil {
		return err
	}
	_, plan, err := activities.Events.LoadPlan(ctx, frozen.Input.EventID, frozen.Input.PlanVersion)
	if err != nil {
		return err
	}
	digest, err := approvalDigest(request, plan)
	if err != nil {
		return err
	}
	if digest != frozen.Input.ApprovalDigest {
		return errors.New("approval digest changed")
	}
	err = activities.Dispatcher.ValidatePlan(ctx, frozen.Input.EventID, frozen.Input.PlanVersion, controlapi.CanonicalFromFrozen(request))
	if errors.Is(err, controlapi.ErrSafetyRejected) {
		return temporal.NewNonRetryableApplicationError(err.Error(), ValidationError, err)
	}
	return err
}

func snapshotDigest(request *gridosv1.OptimizationRequest) ([32]byte, error) {
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func (frozen FrozenEvent) verifySnapshot(request *gridosv1.OptimizationRequest) error {
	digest, err := snapshotDigest(request)
	if err != nil {
		return err
	}
	if digest != frozen.SnapshotDigest {
		return errors.New("frozen snapshot digest changed")
	}
	return nil
}

func approvalDigest(request *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan) ([32]byte, error) {
	options := proto.MarshalOptions{Deterministic: true}
	inputs, err := options.Marshal(request)
	if err != nil {
		return [32]byte{}, err
	}
	planned, err := options.Marshal(plan)
	if err != nil {
		return [32]byte{}, err
	}
	encoded := make([]byte, 8, 8+len(inputs)+len(planned))
	binary.BigEndian.PutUint64(encoded, uint64(len(inputs)))
	encoded = append(encoded, inputs...)
	encoded = append(encoded, planned...)
	return sha256.Sum256(encoded), nil
}

func (activities *Activities) recordPlanningDecision(ctx context.Context, request *gridosv1.OptimizationRequest, action, reason string) error {
	tx, err := activities.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked int
	if err = tx.QueryRow(ctx, `SELECT 1 FROM dispatch_events WHERE event_id = $1 FOR UPDATE`, request.GetEventId()).Scan(&locked); err != nil {
		return err
	}
	var recorded bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_journal WHERE resource_id = $1 AND action = $2 AND new_values->>'plan_version' = $3)`, request.GetEventId(), action, strconv.FormatUint(request.GetPlanVersion(), 10)).Scan(&recorded)
	if err != nil {
		return err
	}
	if recorded {
		return tx.Commit(ctx)
	}
	decision := struct {
		Reason         string `json:"reason,omitempty"`
		FallbackReason string `json:"fallback_reason,omitempty"`
		PlanVersion    uint64 `json:"plan_version"`
	}{PlanVersion: request.GetPlanVersion()}
	if action == "PLAN_FALLBACK_SELECTED" {
		decision.FallbackReason = reason
	} else {
		decision.Reason = reason
	}
	values, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	if err = storage.AppendAudit(ctx, tx, storage.AuditRecord{OccurredAt: activities.Now(), ActorID: "decision", Action: action, ResourceID: request.GetEventId(), NewValues: values, CorrelationID: request.GetCorrelationId()}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (activities *Activities) PersistIntents(ctx context.Context, request PersistInput) error {
	inputs, plan, err := activities.Events.LoadPlan(ctx, request.Input.EventID, request.Input.PlanVersion)
	if err != nil {
		return err
	}
	digest, err := approvalDigest(inputs, plan)
	if err != nil {
		return err
	}
	if digest != request.Input.ApprovalDigest {
		return errors.New("approval digest changed")
	}
	if _, err := activities.Dispatcher.PersistApproved(ctx, request.Input.EventID, request.Input.PlanVersion); err != nil {
		return err
	}
	if request.Launch == nil {
		return errors.New("launch request required")
	}
	_, err = activities.Events.Launch(ctx, request.Launch)
	return err
}

func (activities *Activities) PublishCommands(ctx context.Context, input Input) error {
	for {
		pending, err := activities.unpublishedCount(ctx, input.EventID)
		if err != nil {
			return err
		}
		if pending == 0 {
			break
		}
		if err = activities.Dispatcher.Publish(ctx, nil); err != nil {
			return err
		}
		remaining, err := activities.unpublishedCount(ctx, input.EventID)
		if err != nil {
			return err
		}
		if remaining >= pending {
			return fmt.Errorf("%d commands remain unpublished for event %s", remaining, input.EventID)
		}
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "COMMANDS_PERSISTED", "SENT", "workflow", activities.Now())
	return err
}

func (activities *Activities) unpublishedCount(ctx context.Context, eventID string) (int, error) {
	var pending int
	err := activities.Pool.QueryRow(ctx, `SELECT count(*) FROM command_intents AS intent
		WHERE intent.event_id = $1 AND NOT EXISTS (
			SELECT 1 FROM command_states AS state WHERE state.command_id = intent.command_id
			AND state.state IN ('SENT', 'ACKNOWLEDGED', 'UNCERTAIN', 'REJECTED'))`, eventID).Scan(&pending)
	return pending, err
}

func (activities *Activities) TrackAcknowledgements(ctx context.Context, input Input) error {
	var unresolved int
	err := activities.Pool.QueryRow(ctx, `SELECT count(*) FROM command_intents AS intent
		WHERE intent.event_id = $1 AND NOT EXISTS (
			SELECT 1 FROM command_states AS state WHERE state.command_id = intent.command_id
			AND state.state IN ('ACKNOWLEDGED', 'UNCERTAIN', 'REJECTED'))`, input.EventID).Scan(&unresolved)
	if err != nil {
		return err
	}
	if unresolved != 0 {
		return fmt.Errorf("%d commands lack acknowledgement outcomes", unresolved)
	}
	_, err = activities.Events.Advance(ctx, input.EventID, "SENT", "ACKNOWLEDGED_OR_UNCERTAIN", "workflow", activities.Now())
	return err
}

func (activities *Activities) EndEvent(ctx context.Context, input Input) error {
	rows, err := activities.Pool.Query(ctx, `SELECT DISTINCT ON (device_id) command_id, device_id, plan_version, generation, policy_version, correlation_id
		FROM command_intents WHERE event_id = $1 AND setpoint_kw <> 0
		ORDER BY device_id, generation DESC, issued_at DESC, command_id DESC`, input.EventID)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := activities.Now()
	commands := make([]storage.CommandIntent, 0)
	for rows.Next() {
		var command storage.CommandIntent
		if err = rows.Scan(&command.CommandID, &command.DeviceID, &command.PlanVersion, &command.Generation, &command.PolicyVersion, &command.CorrelationID); err != nil {
			return err
		}
		command.CommandID = fmt.Sprintf("%s-end-%d", command.CommandID, command.Generation+1)
		command.IdempotencyKey = command.CommandID
		command.EventID = input.EventID
		command.Generation++
		command.SetpointKW = 0
		command.IssuedAt = now
		command.EffectiveAt = now
		command.ExpiresAt = now.Add(time.Minute)
		commands = append(commands, command)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, command := range commands {
		if err = storage.InsertZeroCommand(ctx, activities.Pool, command); err != nil {
			return err
		}
	}
	return activities.Dispatcher.Publish(ctx, nil)
}

func (activities *Activities) ProduceReport(ctx context.Context, input Input) error {
	if _, err := reporting.Build(ctx, activities.Reports, input.EventID); err != nil {
		return err
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "RECONCILED", "REPORTED", "workflow", activities.Now())
	return err
}

func (activities *Activities) IssueEmergencyStop(ctx context.Context, command EmergencyCommand) error {
	return activities.insertZeroCommand(ctx, command.EventID, "emergency", command.Generation)
}

func (activities *Activities) ExpireCommands(ctx context.Context, command EmergencyCommand) error {
	return activities.insertZeroCommand(ctx, command.EventID, "expiry", command.Generation)
}

func (activities *Activities) IssueReplacement(ctx context.Context, replacement ReplacementCommand) error {
	var command storage.CommandIntent
	err := activities.Pool.QueryRow(ctx, `SELECT plan_version, setpoint_kw, effective_at, expires_at, policy_version, correlation_id
		FROM command_intents WHERE event_id = $1 ORDER BY generation DESC, command_id LIMIT 1`, replacement.EventID).Scan(
		&command.PlanVersion, &command.SetpointKW, &command.EffectiveAt, &command.ExpiresAt, &command.PolicyVersion, &command.CorrelationID,
	)
	if err != nil {
		return err
	}
	command.CommandID = fmt.Sprintf("%s-%s-%d", replacement.EventID, replacement.DeviceID, replacement.Generation)
	command.IdempotencyKey = command.CommandID
	command.DeviceID = replacement.DeviceID
	command.EventID = replacement.EventID
	command.Generation = int64(replacement.Generation)
	command.IssuedAt = activities.Now()
	if err = storage.InsertCommand(ctx, activities.Pool, command); err != nil {
		return err
	}
	return activities.Dispatcher.Publish(ctx, nil)
}

func (activities *Activities) insertZeroCommand(ctx context.Context, eventID, reason string, generation uint64) error {
	rows, err := activities.Pool.Query(ctx, `SELECT DISTINCT ON (device_id) device_id, plan_version, policy_version, correlation_id
		FROM command_intents WHERE event_id = $1
		ORDER BY device_id, generation DESC, issued_at DESC`, eventID)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := activities.Now()
	commands := make([]storage.CommandIntent, 0)
	for rows.Next() {
		var command storage.CommandIntent
		if err = rows.Scan(&command.DeviceID, &command.PlanVersion, &command.PolicyVersion, &command.CorrelationID); err != nil {
			return err
		}
		command.CommandID = fmt.Sprintf("%s-%s-%s-%d", eventID, reason, command.DeviceID, generation)
		command.IdempotencyKey = command.CommandID
		command.EventID = eventID
		command.Generation = int64(generation)
		command.IssuedAt = now
		command.EffectiveAt = now
		command.ExpiresAt = now.Add(time.Minute)
		commands = append(commands, command)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, command := range commands {
		if err = storage.InsertZeroCommand(ctx, activities.Pool, command); err != nil {
			return err
		}
	}
	return activities.Dispatcher.Publish(ctx, nil)
}
