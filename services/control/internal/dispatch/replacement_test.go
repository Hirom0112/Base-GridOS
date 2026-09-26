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

type replacementSafety struct{ calls int }

func (gate *replacementSafety) Validate(*gridosv1.DispatchPlan, safety.CanonicalState) error {
	gate.calls++
	return nil
}

func TestReplacementStaysInsideApprovedEnvelope(t *testing.T) {
	harness := newActivityHarness(t)
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.EligibilitySnapshot.EligibleDeviceIds = []string{"device-1", "device-2"}
	for _, deviceID := range []string{"device-1", "device-2"} {
		snapshotter.snapshot.Optimization.Devices = append(snapshotter.snapshot.Optimization.Devices, &gridosv1.DeviceState{
			DeviceId: deviceID, UsableEnergyKwh: 10, EnergyKwh: 8, HardwareFloorKwh: 1, EffectiveReserveKwh: 2,
			MaxDischargeKw: 5, DischargeEfficiency: 1, AvailabilityProbability: 1,
			TelemetryObservedAt: timestamppb.New(harness.activities.Now().Add(-time.Second)),
		})
	}
	harness.persist(t)
	require.NoError(t, harness.pool.QueryRow(context.Background(), `UPDATE dispatch_events SET state = 'EXECUTING' WHERE event_id = $1 RETURNING event_id`, harness.input.EventID).Scan(new(string)))
	snapshotter.snapshot.Optimization.PlanVersion = 2
	snapshotter.snapshot.Optimization.Intervals = []*gridosv1.OptimizationInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, TargetKw: 1}}
	snapshotter.snapshot.Canonical.Devices["device-2"] = safety.DeviceState{}
	harness.activities.Dispatcher.Snapshots = snapshotter
	plan := &gridosv1.DispatchPlan{EventId: harness.input.EventID, PlanVersion: 2, PlanId: "replacement-2", SolverVersion: "fallback", ModelVersion: "1", DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-2", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, SetpointKw: 1, ExpectedEnergyKwh: 7}}}}}
	harness.activities.Dispatcher.Optimizer = activityOptimizer{replacementPlan: plan}
	gate := &replacementSafety{}
	harness.activities.Dispatcher.Safety = gate
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 2}))
	require.Equal(t, 1, gate.calls)
	var replacementDevice string
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT device_id FROM command_intents WHERE generation = 2`).Scan(&replacementDevice))
	require.Equal(t, "device-2", replacementDevice)
	require.ErrorContains(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"outside-envelope"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 3}), "dropped device lacks approved schedule")
	var outsideCount int
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT count(*) FROM command_intents WHERE generation = 3`).Scan(&outsideCount))
	require.Zero(t, outsideCount)
}
