package api

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

type dispatchSnapshotter struct {
	steps    *[]string
	snapshot FrozenSnapshot
}

func (source dispatchSnapshotter) Freeze(_ context.Context, _ *gridosv1.DispatchEvent, _ *gridosv1.EventRequest) (FrozenSnapshot, error) {
	*source.steps = append(*source.steps, "freeze")
	return source.snapshot, nil
}

type dispatchOptimizer struct {
	steps *[]string
	plan  *gridosv1.DispatchPlan
}

func (optimizer dispatchOptimizer) Optimize(_ context.Context, _ *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	*optimizer.steps = append(*optimizer.steps, "optimize")
	return optimizer.plan, nil
}

type dispatchSafety struct {
	steps *[]string
	err   error
}

func (gate dispatchSafety) Validate(_ *gridosv1.DispatchPlan, _ safety.CanonicalState) error {
	*gate.steps = append(*gate.steps, "safety")
	return gate.err
}

type dispatchApproval struct {
	steps *[]string
	err   error
}

func (gate dispatchApproval) Require(_ context.Context, _ string, _ uint64) error {
	*gate.steps = append(*gate.steps, "approval")
	return gate.err
}

type dispatchCommands struct {
	steps     *[]string
	persisted []storage.CommandIntent
	published []storage.ClaimedCommand
}

func (commands *dispatchCommands) Persist(_ context.Context, intents []storage.CommandIntent) error {
	*commands.steps = append(*commands.steps, "persist")
	commands.persisted = append(commands.persisted, intents...)
	return nil
}

func (commands *dispatchCommands) Publish(_ context.Context, command storage.ClaimedCommand) error {
	*commands.steps = append(*commands.steps, "publish")
	if len(commands.persisted) == 0 {
		return errors.New("published before persistence")
	}
	commands.published = append(commands.published, command)
	return nil
}

func TestDispatchRunsStraightLineAfterApproval(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	steps := make([]string, 0)
	commands := &dispatchCommands{steps: &steps}
	dispatcher := &Dispatcher{
		Events: NewMemoryEventStore(),
		Snapshots: dispatchSnapshotter{steps: &steps, snapshot: FrozenSnapshot{Optimization: &gridosv1.OptimizationRequest{
			EventId: "event-1", PlanVersion: 2, ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"}, CorrelationId: "correlation-1",
		}}},
		Optimizer: dispatchOptimizer{steps: &steps, plan: &gridosv1.DispatchPlan{
			EventId: "event-1", PlanVersion: 2, CreatedAt: timestamp(now),
			DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: timestamp(now.Add(time.Minute)), EndTime: timestamp(now.Add(time.Hour)), SetpointKw: 3.5}}}},
		}},
		Safety:   dispatchSafety{steps: &steps},
		Approval: dispatchApproval{steps: &steps},
		Commands: commands,
		Now:      func() time.Time { return now },
	}
	request := &gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{RequestId: "event-1"}, IdempotencyKey: "create-event-1"}
	if _, err := dispatcher.Dispatch(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if want := []string{"freeze", "optimize", "safety", "approval", "persist", "publish"}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	if len(commands.persisted) != 1 || len(commands.published) != 1 || commands.persisted[0].CommandID != commands.published[0].CommandID {
		t.Fatalf("persisted = %#v, published = %#v", commands.persisted, commands.published)
	}
	if DispatcherLifecycle != "REPLACED-IN-WAVE-2" {
		t.Fatalf("lifecycle = %q", DispatcherLifecycle)
	}
}

func TestDispatchStopsBeforePersistenceWithoutApproval(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	steps := make([]string, 0)
	commands := &dispatchCommands{steps: &steps}
	dispatcher := &Dispatcher{
		Events:    NewMemoryEventStore(),
		Snapshots: dispatchSnapshotter{steps: &steps, snapshot: FrozenSnapshot{Optimization: &gridosv1.OptimizationRequest{EventId: "event-2"}}},
		Optimizer: dispatchOptimizer{steps: &steps, plan: &gridosv1.DispatchPlan{EventId: "event-2", PlanVersion: 1}},
		Safety:    dispatchSafety{steps: &steps}, Approval: dispatchApproval{steps: &steps, err: errors.New("not approved")}, Commands: commands, Now: func() time.Time { return now },
	}
	request := &gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{RequestId: "event-2"}, IdempotencyKey: "create-event-2"}
	_, err := dispatcher.Dispatch(context.Background(), request)
	if !errors.Is(err, ErrApprovalRequired) || len(commands.persisted) != 0 || len(commands.published) != 0 {
		t.Fatalf("dispatch error = %v, persisted = %d, published = %d", err, len(commands.persisted), len(commands.published))
	}
}
