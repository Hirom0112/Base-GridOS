package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

var (
	ErrApprovalRequired = errors.New("approved plan required")
	ErrSafetyRejected   = errors.New("safety gate rejected plan")
)

type FrozenSnapshot struct {
	Optimization *gridosv1.OptimizationRequest
	Canonical    safety.CanonicalState
}

type Snapshotter interface {
	Freeze(context.Context, *gridosv1.DispatchEvent, *gridosv1.EventRequest) (FrozenSnapshot, error)
}

type Optimizer interface {
	Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error)
	Forecast(context.Context, *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error)
	Replace(context.Context, *gridosv1.ReplaceRequest) (*gridosv1.ReplaceResponse, error)
}

type SafetyGate interface {
	Validate(*gridosv1.DispatchPlan, safety.CanonicalState) error
}

type ApprovalGate interface {
	Require(context.Context, string, uint64) error
}

type IntentPipeline interface {
	Persist(context.Context, []storage.CommandIntent) error
	Publish(context.Context, storage.ClaimedCommand) error
}

type Dispatcher struct {
	Events    EventStore
	Snapshots Snapshotter
	Optimizer Optimizer
	Safety    SafetyGate
	Approval  ApprovalGate
	Commands  IntentPipeline
	Now       func() time.Time
}

type LifecycleStore interface {
	EventStore
	StorePlanned(context.Context, string, *gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, time.Time) (*gridosv1.DispatchEvent, error)
	ValidatePlanned(context.Context, string, uint64, []storage.StoredViolation, time.Time) (*gridosv1.DispatchEvent, error)
	Advance(context.Context, string, string, string, string, time.Time) (*gridosv1.DispatchEvent, error)
	Violations(context.Context, string) ([]storage.StoredViolation, error)
}

func (dispatcher *Dispatcher) Freeze(ctx context.Context, eventID string, request *gridosv1.EventRequest) (FrozenSnapshot, error) {
	event, _, err := dispatcher.Events.Get(ctx, eventID)
	if err != nil {
		return FrozenSnapshot{}, err
	}
	return dispatcher.Snapshots.Freeze(ctx, event, request)
}

func (dispatcher *Dispatcher) RequestPlan(ctx context.Context, snapshot FrozenSnapshot) (*gridosv1.DispatchPlan, error) {
	store, ok := dispatcher.Events.(LifecycleStore)
	if !ok {
		return nil, errors.New("lifecycle store required")
	}
	plan, err := dispatcher.Optimizer.Optimize(ctx, snapshot.Optimization)
	if err != nil {
		return nil, err
	}
	if plan == nil || plan.GetPlanVersion() != snapshot.Optimization.GetPlanVersion() {
		return nil, errors.New("optimizer returned the wrong plan version")
	}
	_, err = store.StorePlanned(ctx, plan.GetEventId(), snapshot.Optimization, plan, dispatcher.Now())
	return plan, err
}

func (dispatcher *Dispatcher) ValidatePlan(ctx context.Context, eventID string, planVersion uint64, canonical safety.CanonicalState) error {
	store, ok := dispatcher.Events.(LifecycleStore)
	if !ok {
		return errors.New("lifecycle store required")
	}
	_, plan, err := store.LoadPlan(ctx, eventID, planVersion)
	if err != nil {
		return err
	}
	validateErr := dispatcher.Safety.Validate(plan, canonical)
	violations := storedViolations(validateErr)
	_, err = store.ValidatePlanned(ctx, eventID, planVersion, violations, dispatcher.Now())
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return errors.Join(ErrSafetyRejected, validateErr)
	}
	return nil
}

func storedViolations(validateErr error) []storage.StoredViolation {
	stored := make([]storage.StoredViolation, 0)
	var rejection SafetyRejection
	switch {
	case validateErr == nil:
	case errors.As(validateErr, &rejection):
		for _, violation := range rejection.Violations {
			stored = append(stored, storage.StoredViolation{Code: string(violation.Code)})
		}
	default:
		stored = append(stored, storage.StoredViolation{Code: string(safety.ContradictoryInput)})
	}
	return stored
}

func (dispatcher *Dispatcher) Plan(ctx context.Context, request *gridosv1.CreateEventRequestRequest) (*gridosv1.DispatchEvent, error) {
	event, err := dispatcher.Events.Create(ctx, request.GetEventRequest(), request.GetIdempotencyKey(), dispatcher.Now())
	if err != nil {
		return nil, err
	}
	if event.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED {
		return event, nil
	}
	snapshot, err := dispatcher.Freeze(ctx, event.GetEventId(), request.GetEventRequest())
	if err != nil {
		return nil, err
	}
	plan, err := dispatcher.RequestPlan(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	if err = dispatcher.ValidatePlan(ctx, event.GetEventId(), plan.GetPlanVersion(), snapshot.Canonical); err != nil && !errors.Is(err, ErrSafetyRejected) {
		return nil, err
	}
	validated, _, loadErr := dispatcher.Events.Get(ctx, event.GetEventId())
	return validated, loadErr
}

func (dispatcher *Dispatcher) PersistApproved(ctx context.Context, eventID string, planVersion uint64) ([]storage.CommandIntent, error) {
	store, ok := dispatcher.Events.(LifecycleStore)
	if !ok {
		return nil, errors.New("lifecycle store required")
	}
	if err := dispatcher.Approval.Require(ctx, eventID, planVersion); err != nil {
		return nil, err
	}
	request, plan, err := store.LoadPlan(ctx, eventID, planVersion)
	if err != nil {
		return nil, err
	}
	commands, err := commandIntents(plan, request)
	if err != nil {
		return nil, err
	}
	if err = dispatcher.Commands.Persist(ctx, commands); err != nil {
		return nil, err
	}
	return commands, nil
}

func (dispatcher *Dispatcher) Publish(ctx context.Context, commands []storage.CommandIntent) error {
	if batch, ok := dispatcher.Commands.(interface{ PublishAll(context.Context) error }); ok {
		return batch.PublishAll(ctx)
	}
	for _, command := range commands {
		if err := dispatcher.Commands.Publish(ctx, storage.ClaimedCommand{CommandIntent: command}); err != nil {
			return err
		}
	}
	return nil
}

type SafetyRejection struct {
	Violations []safety.Violation
}

func (rejection SafetyRejection) Error() string {
	return fmt.Sprintf("%v: %v", ErrSafetyRejected, rejection.Violations)
}

func (SafetyRejection) Unwrap() error {
	return ErrSafetyRejected
}

type IndependentSafetyGate struct{}

func (IndependentSafetyGate) Validate(plan *gridosv1.DispatchPlan, canonical safety.CanonicalState) error {
	safetyPlan, err := safetyPlan(plan, canonical)
	if err != nil {
		return err
	}
	approval, violations := safety.Validate(safetyPlan, canonical)
	if approval.Approved {
		return nil
	}
	return SafetyRejection{Violations: violations}
}

func commandIntents(plan *gridosv1.DispatchPlan, request *gridosv1.OptimizationRequest) ([]storage.CommandIntent, error) {
	commands := make([]storage.CommandIntent, 0)
	for _, schedule := range plan.GetDeviceSchedules() {
		for index, interval := range schedule.GetIntervals() {
			if interval.GetBeginTime() == nil || interval.GetEndTime() == nil {
				return nil, errors.New("schedule interval times required")
			}
			id := fmt.Sprintf("%s-%s-%d", plan.GetEventId(), schedule.GetDeviceId(), index)
			commands = append(commands, storage.CommandIntent{
				CommandID: id, IdempotencyKey: id, DeviceID: schedule.GetDeviceId(), EventID: plan.GetEventId(),
				PlanVersion: int64(plan.GetPlanVersion()), Generation: int64(plan.GetPlanVersion()), SetpointKW: interval.GetSetpointKw(),
				IssuedAt: plan.GetCreatedAt().AsTime(), EffectiveAt: interval.GetBeginTime().AsTime(), ExpiresAt: interval.GetEndTime().AsTime(),
				PolicyVersion: request.GetReservePolicy().GetPolicyVersion(), CorrelationID: request.GetCorrelationId(),
			})
		}
	}
	return commands, nil
}

func safetyPlan(plan *gridosv1.DispatchPlan, canonical safety.CanonicalState) (safety.Plan, error) {
	requestPlan := safety.Plan{Boundary: canonical.Boundary, PolicyVersion: canonical.PolicyVersion, Generation: canonical.ExpectedGeneration}
	if len(plan.GetDeviceSchedules()) == 0 {
		return emptyShortfallPlan(requestPlan, plan.GetShortfalls())
	}
	for _, schedule := range plan.GetDeviceSchedules() {
		state, found := canonical.Devices[schedule.GetDeviceId()]
		if !found || state.EnergyKWh == nil {
			return requestPlan, errors.New("canonical device state required")
		}
		device := safety.DevicePlan{DeviceID: schedule.GetDeviceId(), EnergyKWh: []float64{*state.EnergyKWh}}
		switch schedule.GetReserveSelection() {
		case gridosv1.ReserveSelection_RESERVE_SELECTION_BASE:
			device.ReserveSelection = safety.BaseReserveSelection
		case gridosv1.ReserveSelection_RESERVE_SELECTION_TRAVEL_FLEX:
			device.ReserveSelection = safety.TravelFlexReserveSelection
		case gridosv1.ReserveSelection_RESERVE_SELECTION_UNSPECIFIED:
		default:
			return requestPlan, errors.New("unknown reserve selection")
		}
		device.SelectedReserveKWh = schedule.GetSelectedReserveKwh()
		for _, interval := range schedule.GetIntervals() {
			if interval.GetBeginTime() == nil || interval.GetEndTime() == nil {
				return requestPlan, errors.New("schedule interval times required")
			}
			if requestPlan.Interval == 0 {
				requestPlan.EffectiveAt = interval.GetBeginTime().AsTime()
				requestPlan.Interval = interval.GetEndTime().AsTime().Sub(requestPlan.EffectiveAt)
			}
			requestPlan.ExpiresAt = interval.GetEndTime().AsTime()
			setpoint := interval.GetSetpointKw()
			device.ChargeKW = append(device.ChargeKW, math.Max(-setpoint, 0))
			device.DischargeKW = append(device.DischargeKW, math.Max(setpoint, 0))
			device.MeterExportKW = append(device.MeterExportKW, setpoint)
			device.EnergyKWh = append(device.EnergyKWh, interval.GetExpectedEnergyKwh())
		}
		requestPlan.Devices = append(requestPlan.Devices, device)
	}
	for _, shortfall := range plan.GetShortfalls() {
		requestPlan.DeclaredShortfall = max(requestPlan.DeclaredShortfall, shortfall.GetShortfallKw())
		requestPlan.TargetKW = max(requestPlan.TargetKW, shortfall.GetRequestedKw())
	}
	return requestPlan, nil
}

func emptyShortfallPlan(requestPlan safety.Plan, shortfalls []*gridosv1.ShortfallReport) (safety.Plan, error) {
	if len(shortfalls) == 0 {
		return requestPlan, errors.New("device schedules or quantified shortfall required")
	}
	for index, shortfall := range shortfalls {
		begin, end := shortfall.GetIntervalBeginTime(), shortfall.GetIntervalEndTime()
		if begin == nil || end == nil || begin.CheckValid() != nil || end.CheckValid() != nil {
			return requestPlan, errors.New("shortfall interval times required")
		}
		start, finish := begin.AsTime(), end.AsTime()
		if !finish.After(start) || index > 0 && (!start.Equal(requestPlan.ExpiresAt) || finish.Sub(start) != requestPlan.Interval) {
			return requestPlan, errors.New("contiguous shortfall intervals required")
		}
		if !(shortfall.GetRequestedKw() >= 0 && shortfall.GetFeasibleKw() == 0 && shortfall.GetShortfallKw() >= 0 && math.Abs(shortfall.GetRequestedKw()-shortfall.GetShortfallKw()) <= 1e-9) {
			return requestPlan, errors.New("empty plan requires full quantified shortfall")
		}
		if index == 0 {
			requestPlan.EffectiveAt = start
			requestPlan.Interval = finish.Sub(start)
		}
		requestPlan.ExpiresAt = finish
		requestPlan.TargetKW = max(requestPlan.TargetKW, shortfall.GetRequestedKw())
		requestPlan.DeclaredShortfall = max(requestPlan.DeclaredShortfall, shortfall.GetShortfallKw())
	}
	return requestPlan, nil
}
