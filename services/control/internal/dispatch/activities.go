package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	reporting "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
)

const ReconciliationActivitiesStatus = "STUBBED"

type Activities struct {
	Dispatcher *controlapi.Dispatcher
	Events     controlapi.LifecycleStore
	Pool       *pgxpool.Pool
	Reports    reporting.Source
	Now        func() time.Time
}

type FrozenEvent struct {
	Input    Input
	Snapshot controlapi.FrozenSnapshot
}

func (activities *Activities) FreezeInputs(ctx context.Context, input Input) (FrozenEvent, error) {
	if input.Request == nil {
		return FrozenEvent{}, errors.New("event request required")
	}
	snapshot, err := activities.Dispatcher.Freeze(ctx, input.EventID, input.Request)
	return FrozenEvent{Input: input, Snapshot: snapshot}, err
}

func (activities *Activities) RequestPlan(ctx context.Context, frozen FrozenEvent) (FrozenEvent, error) {
	plan, err := activities.Dispatcher.RequestPlan(ctx, frozen.Snapshot)
	if err != nil {
		return frozen, err
	}
	frozen.Input.PlanVersion = plan.GetPlanVersion()
	return frozen, nil
}

func (activities *Activities) ValidatePlan(ctx context.Context, frozen FrozenEvent) error {
	err := activities.Dispatcher.ValidatePlan(ctx, frozen.Input.EventID, frozen.Input.PlanVersion, frozen.Snapshot.Canonical)
	if errors.Is(err, controlapi.ErrSafetyRejected) {
		return temporal.NewNonRetryableApplicationError(err.Error(), ValidationError, err)
	}
	return err
}

func (activities *Activities) PersistIntents(ctx context.Context, input Input) error {
	if _, err := activities.Dispatcher.PersistApproved(ctx, input.EventID, input.PlanVersion); err != nil {
		return err
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "APPROVED", "COMMANDS_PERSISTED", "workflow", activities.Now())
	return err
}

func (activities *Activities) PublishCommands(ctx context.Context, input Input) error {
	if err := activities.Dispatcher.Publish(ctx, nil); err != nil {
		return err
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "COMMANDS_PERSISTED", "SENT", "workflow", activities.Now())
	return err
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

func (activities *Activities) VerifyDelivery(ctx context.Context, input Input) error {
	if _, err := activities.Events.Advance(ctx, input.EventID, "ACKNOWLEDGED_OR_UNCERTAIN", "EXECUTING", "workflow", activities.Now()); err != nil {
		return err
	}
	_, err := activities.Events.Advance(ctx, input.EventID, "EXECUTING", "VERIFIED", "workflow", activities.Now())
	return err
}

func (activities *Activities) EndEvent(ctx context.Context, input Input) error {
	rows, err := activities.Pool.Query(ctx, `SELECT command_id, device_id, plan_version, generation, expires_at, policy_version, correlation_id
		FROM command_intents WHERE event_id = $1 ORDER BY command_id`, input.EventID)
	if err != nil {
		return err
	}
	defer rows.Close()
	now := activities.Now()
	commands := make([]storage.CommandIntent, 0)
	for rows.Next() {
		var command storage.CommandIntent
		if err = rows.Scan(&command.CommandID, &command.DeviceID, &command.PlanVersion, &command.Generation, &command.ExpiresAt, &command.PolicyVersion, &command.CorrelationID); err != nil {
			return err
		}
		command.CommandID = fmt.Sprintf("%s-end-%d", command.CommandID, command.Generation+1)
		command.IdempotencyKey = command.CommandID
		command.EventID = input.EventID
		command.Generation++
		command.SetpointKW = 0
		command.IssuedAt = now
		command.EffectiveAt = now
		commands = append(commands, command)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, command := range commands {
		if err = storage.InsertCommand(ctx, activities.Pool, command); err != nil {
			return err
		}
	}
	return activities.Dispatcher.Publish(ctx, nil)
}

func (activities *Activities) ReconcileLateMessages(ctx context.Context, input Input) error {
	_, err := activities.Events.Advance(ctx, input.EventID, "VERIFIED", "RECONCILED", "workflow", activities.Now())
	return err
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
	now := activities.Now()
	command := storage.CommandIntent{
		CommandID: fmt.Sprintf("%s-%s-%d", eventID, reason, generation), IdempotencyKey: fmt.Sprintf("%s-%s-%d", eventID, reason, generation),
		DeviceID: "event", EventID: eventID, PlanVersion: int64(generation), Generation: int64(generation),
		IssuedAt: now, EffectiveAt: now, ExpiresAt: now.Add(time.Minute), CorrelationID: eventID,
	}
	if err := storage.InsertCommand(ctx, activities.Pool, command); err != nil {
		return err
	}
	return activities.Dispatcher.Publish(ctx, nil)
}
