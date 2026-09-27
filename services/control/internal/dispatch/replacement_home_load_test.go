package dispatch

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type siteLoadOptimizer struct {
	activityOptimizer
	siteLoads []*gridosv1.ForecastSiteLoad
	replaced  *gridosv1.ReplaceRequest
}

func (optimizer *siteLoadOptimizer) Forecast(context.Context, *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error) {
	return &gridosv1.ForecastResponse{SiteLoads: optimizer.siteLoads}, nil
}

func (optimizer *siteLoadOptimizer) Replace(ctx context.Context, request *gridosv1.ReplaceRequest) (*gridosv1.ReplaceResponse, error) {
	optimizer.replaced = request
	return optimizer.activityOptimizer.Replace(ctx, request)
}

type canonicalCaptureSafety struct {
	canonical safety.CanonicalState
}

func (gate *canonicalCaptureSafety) Validate(_ *gridosv1.DispatchPlan, canonical safety.CanonicalState) error {
	gate.canonical = canonical
	return nil
}

func TestReplacementCarriesFrozenSiteLoadsIntoOptimizationAndSafety(t *testing.T) {
	harness := newActivityHarness(t)
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.EligibilitySnapshot.EligibleDeviceIds = []string{"device-1", "device-2"}
	for _, deviceID := range []string{"device-1", "device-2"} {
		snapshotter.snapshot.Optimization.Devices = append(snapshotter.snapshot.Optimization.Devices, &gridosv1.DeviceState{
			DeviceId: deviceID, SiteId: "site-" + deviceID, UsableEnergyKwh: 10, EnergyKwh: 8, HardwareFloorKwh: 1, EffectiveReserveKwh: 2,
			MaxDischargeKw: 5, DischargeEfficiency: 1, AvailabilityProbability: 1,
			TelemetryObservedAt: timestamppb.New(harness.activities.Now().Add(-time.Second)),
		})
	}
	harness.persist(t)
	require.NoError(t, harness.pool.QueryRow(context.Background(), `UPDATE dispatch_events SET state = 'EXECUTING' WHERE event_id = $1 RETURNING event_id`, harness.input.EventID).Scan(new(string)))
	snapshotter.snapshot.Optimization.PlanVersion = 2
	snapshotter.snapshot.Optimization.MeasurementBoundary = gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT
	begin, end := harness.input.Request.BeginTime, harness.input.Request.EndTime
	snapshotter.snapshot.Optimization.Intervals = []*gridosv1.OptimizationInterval{{BeginTime: begin, EndTime: end, TargetKw: 1}}
	snapshotter.snapshot.Canonical.Devices["device-2"] = safety.DeviceState{}
	harness.activities.Dispatcher.Snapshots = snapshotter
	window := end.AsTime().Sub(begin.AsTime()).Hours()
	plan := &gridosv1.DispatchPlan{EventId: harness.input.EventID, PlanVersion: 2, PlanId: "replacement-2", SolverVersion: "fallback", ModelVersion: "1", DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-2", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: begin, EndTime: end, SetpointKw: 1, ExpectedEnergyKwh: 7}}}}}
	optimizer := &siteLoadOptimizer{
		activityOptimizer: activityOptimizer{replacementPlan: plan},
		siteLoads:         []*gridosv1.ForecastSiteLoad{{SiteId: "site-device-2", IntervalBeginTime: begin, LoadKwh: &gridosv1.ForecastValue{Value: 1.5 * window, Upper: 2 * window}}},
	}
	harness.activities.Dispatcher.Optimizer = optimizer
	gate := &canonicalCaptureSafety{}
	harness.activities.Dispatcher.Safety = gate
	replacement := ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 2}
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), replacement))
	require.Len(t, optimizer.replaced.GetCurrent().GetForecast().GetSiteLoads(), 1)
	require.Equal(t, "site-device-2", optimizer.replaced.GetCurrent().GetForecast().GetSiteLoads()[0].GetSiteId())
	require.InDelta(t, 2.0, gate.canonical.Devices["device-2"].HomeLoadKW, 1e-9)
	require.Equal(t, int64(2), gate.canonical.ExpectedGeneration)
}
