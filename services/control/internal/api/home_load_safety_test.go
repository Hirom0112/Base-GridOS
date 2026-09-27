package api

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func homeLoadFixture(boundary gridosv1.MeasurementBoundary, loads []*gridosv1.ForecastSiteLoad) (*Dispatcher, *recordingLifecycleStore, *gridosv1.OptimizationRequest) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	begin, end := now.Add(time.Minute), now.Add(time.Hour+time.Minute)
	base := 8.0
	frozen := &gridosv1.OptimizationRequest{
		RequestedAt: timestamppb.New(now), PlanVersion: 1, MeasurementBoundary: boundary, ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"},
		Intervals: []*gridosv1.OptimizationInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end), TargetKw: 0.9}},
		Devices: []*gridosv1.DeviceState{{DeviceId: "device-1", SiteId: "site-1", UsableEnergyKwh: 10, EnergyKwh: 9, HardwareFloorKwh: 5, EffectiveReserveKwh: 8, BaseReserveKwh: &base,
			MaxChargeKw: 2, MaxDischargeKw: 2, ChargeEfficiency: 0.95, DischargeEfficiency: 0.95, AvailabilityProbability: 1, TelemetryObservedAt: timestamppb.New(now)}},
		Forecast: &gridosv1.ForecastResponse{SiteLoads: loads},
	}
	plan := &gridosv1.DispatchPlan{PlanVersion: 1, DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1", ReserveSelection: gridosv1.ReserveSelection_RESERVE_SELECTION_BASE, SelectedReserveKwh: 8,
		Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end), SetpointKw: 0.9, ExpectedEnergyKwh: 9 - 0.9/0.95}}}}}
	store := &recordingLifecycleStore{MemoryEventStore: NewMemoryEventStore(), frozen: frozen, plan: plan}
	return &Dispatcher{Events: store, Safety: IndependentSafetyGate{}, Now: func() time.Time { return now }}, store, frozen
}

func siteLoad(begin time.Time, value, upper float64) *gridosv1.ForecastSiteLoad {
	return &gridosv1.ForecastSiteLoad{SiteId: "site-1", IntervalBeginTime: timestamppb.New(begin), LoadKwh: &gridosv1.ForecastValue{Value: value, Lower: 0, Upper: upper}}
}

func TestValidatePlanRejectsReserveBreachFromFrozenHomeLoad(t *testing.T) {
	begin := time.Date(2026, 9, 26, 12, 1, 0, 0, time.UTC)
	loads := []*gridosv1.ForecastSiteLoad{siteLoad(begin.Add(-time.Hour), 0.1, 0.1), siteLoad(begin, 0.8, 1.0)}

	dispatcher, store, frozen := homeLoadFixture(gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_BATTERY_TERMINAL, loads)
	require.NoError(t, dispatcher.ValidatePlan(context.Background(), "event-1", 1, CanonicalFromFrozen(frozen)))
	require.Empty(t, store.violations)

	dispatcher, store, frozen = homeLoadFixture(gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, loads)
	err := dispatcher.ValidatePlan(context.Background(), "event-1", 1, CanonicalFromFrozen(frozen))

	require.ErrorIs(t, err, ErrSafetyRejected)
	require.Contains(t, store.violations, storage.StoredViolation{Code: string(safety.EnergyBelowReserve)})
}

func TestValidatePlanRejectsDispatchedDeviceWithoutFrozenHomeLoad(t *testing.T) {
	begin := time.Date(2026, 9, 26, 12, 1, 0, 0, time.UTC)
	loads := []*gridosv1.ForecastSiteLoad{siteLoad(begin.Add(time.Minute), 0.1, 0.1)}
	dispatcher, store, frozen := homeLoadFixture(gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, loads)

	err := dispatcher.ValidatePlan(context.Background(), "event-1", 1, CanonicalFromFrozen(frozen))

	require.ErrorIs(t, err, ErrSafetyRejected)
	require.Contains(t, store.violations, storage.StoredViolation{Code: string(safety.ContradictoryInput)})
}
