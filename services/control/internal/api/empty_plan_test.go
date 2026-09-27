package api

import (
	"context"
	"errors"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEmptyPlanShortfall(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	canonical := safety.CanonicalState{Now: now, Boundary: safety.MeterNetExport, PolicyVersion: "policy-1", ExpectedGeneration: 1}
	shortfall := &gridosv1.ShortfallReport{
		IntervalBeginTime: timestamppb.New(now.Add(time.Minute)),
		IntervalEndTime:   timestamppb.New(now.Add(6 * time.Minute)),
		RequestedKw:       5,
		FeasibleKw:        0,
		ShortfallKw:       5,
	}
	plan := &gridosv1.DispatchPlan{Shortfalls: []*gridosv1.ShortfallReport{shortfall}}
	if err := (IndependentSafetyGate{}).Validate(plan, canonical); err != nil {
		t.Fatalf("full declared shortfall rejected: %v", err)
	}
	shortfall.ShortfallKw = 4
	if err := (IndependentSafetyGate{}).Validate(plan, canonical); err == nil {
		t.Fatal("underdeclared shortfall approved")
	}
	shortfall.ShortfallKw = 5
	plan.Shortfalls = append(plan.Shortfalls, &gridosv1.ShortfallReport{
		IntervalBeginTime: timestamppb.New(now.Add(6 * time.Minute)),
		IntervalEndTime:   timestamppb.New(now.Add(11 * time.Minute)),
		RequestedKw:       5,
		ShortfallKw:       4,
	})
	if err := (IndependentSafetyGate{}).Validate(plan, canonical); err == nil {
		t.Fatal("partially declared second interval approved")
	}
	plan.Shortfalls = nil
	if err := (IndependentSafetyGate{}).Validate(plan, canonical); err == nil {
		t.Fatal("unquantified empty plan approved")
	}
}

type recordingLifecycleStore struct {
	*MemoryEventStore
	frozen     *gridosv1.OptimizationRequest
	plan       *gridosv1.DispatchPlan
	violations []storage.StoredViolation
}

func (store *recordingLifecycleStore) LoadPlan(context.Context, string, uint64) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return store.frozen, store.plan, nil
}

func (store *recordingLifecycleStore) StorePlanned(context.Context, string, *gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, time.Time) (*gridosv1.DispatchEvent, error) {
	return nil, errors.New("not used")
}

func (store *recordingLifecycleStore) ValidatePlanned(_ context.Context, _ string, _ uint64, violations []storage.StoredViolation, _ time.Time) (*gridosv1.DispatchEvent, error) {
	store.violations = violations
	return &gridosv1.DispatchEvent{}, nil
}

func (store *recordingLifecycleStore) Advance(context.Context, string, string, string, string, time.Time) (*gridosv1.DispatchEvent, error) {
	return nil, errors.New("not used")
}

func (store *recordingLifecycleStore) Violations(context.Context, string) ([]storage.StoredViolation, error) {
	return store.violations, nil
}

func TestValidatePlanStoresEachTypedViolation(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	begin, end := now.Add(time.Minute), now.Add(time.Hour+time.Minute)
	base := 8.0
	frozen := &gridosv1.OptimizationRequest{
		RequestedAt: timestamppb.New(now), PlanVersion: 1, ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"},
		Devices: []*gridosv1.DeviceState{{DeviceId: "device-1", UsableEnergyKwh: 10, EnergyKwh: 9, HardwareFloorKwh: 5, EffectiveReserveKwh: 8, BaseReserveKwh: &base,
			MaxChargeKw: 2, MaxDischargeKw: 2, ChargeEfficiency: 0.95, DischargeEfficiency: 0.95, AvailabilityProbability: 1, TelemetryObservedAt: timestamppb.New(now)}},
	}
	plan := &gridosv1.DispatchPlan{PlanVersion: 1, DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1", ReserveSelection: gridosv1.ReserveSelection_RESERVE_SELECTION_BASE, SelectedReserveKwh: 8,
		Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end), SetpointKw: 1.9, ExpectedEnergyKwh: 7}}}}}
	store := &recordingLifecycleStore{MemoryEventStore: NewMemoryEventStore(), frozen: frozen, plan: plan}
	dispatcher := &Dispatcher{Events: store, Safety: IndependentSafetyGate{}, Now: func() time.Time { return now }}

	err := dispatcher.ValidatePlan(context.Background(), "event-1", 1, CanonicalFromFrozen(frozen))

	require.ErrorIs(t, err, ErrSafetyRejected)
	require.Contains(t, store.violations, storage.StoredViolation{Code: string(safety.EnergyBelowReserve)})
	for _, violation := range store.violations {
		require.Regexp(t, `^[A-Z_]+$`, violation.Code)
	}

	plan.DeviceSchedules[0].Intervals[0].SetpointKw = 0.95
	plan.DeviceSchedules[0].Intervals[0].ExpectedEnergyKwh = 8
	require.NoError(t, dispatcher.ValidatePlan(context.Background(), "event-1", 1, CanonicalFromFrozen(frozen)))
	require.Empty(t, store.violations)
}
