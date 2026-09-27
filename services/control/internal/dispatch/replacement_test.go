package dispatch

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

type replacementSafety struct {
	calls int
	err   error
}

func TestReplacementPublishesConsecutiveDeviceGenerations(t *testing.T) {
	harness := newActivityHarness(t)
	harness.plan(t)
	now := harness.activities.Now()
	commands := []storage.CommandIntent{
		{CommandID: "replacement-z", IdempotencyKey: "replacement-z", DeviceID: "device-2", EventID: harness.input.EventID, PlanVersion: 1, SetpointKW: 1, IssuedAt: now, EffectiveAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Hour), PolicyVersion: "policy-1", CorrelationID: "correlation-1"},
		{CommandID: "replacement-a", IdempotencyKey: "replacement-a", DeviceID: "device-2", EventID: harness.input.EventID, PlanVersion: 1, SetpointKW: 1, IssuedAt: now, EffectiveAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Hour), PolicyVersion: "policy-1", CorrelationID: "correlation-1"},
	}
	require.NoError(t, harness.activities.publishCommands(context.Background(), commands))
	var acknowledged int
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT count(*) FROM command_acknowledgements WHERE command_id IN ($1, $2)`, commands[0].CommandID, commands[1].CommandID).Scan(&acknowledged))
	require.Equal(t, 2, acknowledged)
}

func (gate *replacementSafety) Validate(*gridosv1.DispatchPlan, safety.CanonicalState) error {
	gate.calls++
	return gate.err
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
	optimizer := &traceCaptureOptimizer{activityOptimizer: activityOptimizer{replacementPlan: plan}, identities: make(map[string][2]string)}
	harness.activities.Dispatcher.Optimizer = optimizer
	gate := &replacementSafety{}
	harness.activities.Dispatcher.Safety = gate
	replacement := ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 2}
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), replacement))
	require.Equal(t, [2]string{"correlation-1", "event-1"}, optimizer.identities["Replace"])
	require.Equal(t, 1, gate.calls)
	var replacementDevice string
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT device_id FROM command_intents WHERE plan_version = 2`).Scan(&replacementDevice))
	require.Equal(t, "device-2", replacementDevice)
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), replacement))
	require.Equal(t, 2, harness.count(t, "plan_versions"))
	require.Equal(t, 2, harness.count(t, "command_intents"))
	require.ErrorContains(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"outside-envelope"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 3}), "dropped device lacks approved schedule")
	var outsideCount int
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT count(*) FROM command_intents WHERE plan_version = 3`).Scan(&outsideCount))
	require.Zero(t, outsideCount)
}

func TestReplacementSafetyRejectionLeavesQuantifiedShortfall(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	require.NoError(t, harness.pool.QueryRow(context.Background(), `UPDATE dispatch_events SET state = 'EXECUTING' WHERE event_id = $1 RETURNING event_id`, harness.input.EventID).Scan(new(string)))
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.PlanVersion = 2
	snapshotter.snapshot.Optimization.Intervals = []*gridosv1.OptimizationInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, TargetKw: 1}}
	harness.activities.Dispatcher.Snapshots = snapshotter
	harness.activities.Dispatcher.Optimizer = activityOptimizer{replacementPlan: &gridosv1.DispatchPlan{
		EventId: harness.input.EventID, PlanVersion: 2, PlanId: "unsafe-replacement", SolverVersion: "fallback", ModelVersion: "1",
		DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-2", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, SetpointKw: 1}}}},
	}}
	gate := &replacementSafety{err: errors.New("reserve")}
	harness.activities.Dispatcher.Safety = gate

	require.NoError(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{
		EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 2,
	}))
	require.Equal(t, 1, gate.calls)
	require.Equal(t, 1, harness.count(t, "command_intents"))
	_, plan, err := harness.events.LoadPlan(context.Background(), harness.input.EventID, 2)
	require.NoError(t, err)
	require.Empty(t, plan.GetDeviceSchedules())
	require.Equal(t, 1.0, plan.GetShortfalls()[0].GetShortfallKw())
	var decisions int
	require.NoError(t, harness.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_journal WHERE action = 'REPLACEMENT_SAFETY_REJECTED' AND resource_id = $1`, harness.input.EventID).Scan(&decisions))
	require.Equal(t, 1, decisions)
}

func TestReplacementRetainsSurvivingApprovedSchedules(t *testing.T) {
	harness := newActivityHarness(t)
	approved := harness.activities.Dispatcher.Optimizer.(activityOptimizer).plan
	approved.DeviceSchedules = append(approved.DeviceSchedules, &gridosv1.DeviceSchedule{DeviceId: "device-2", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, SetpointKw: 1}}})
	approved.Shortfalls = []*gridosv1.ShortfallReport{{IntervalBeginTime: harness.input.Request.BeginTime, IntervalEndTime: harness.input.Request.EndTime, RequestedKw: 2, FeasibleKw: 2}}
	harness.persist(t)
	require.NoError(t, harness.pool.QueryRow(context.Background(), `UPDATE dispatch_events SET state = 'EXECUTING' WHERE event_id = $1 RETURNING event_id`, harness.input.EventID).Scan(new(string)))
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.PlanVersion = 2
	snapshotter.snapshot.Optimization.Intervals = []*gridosv1.OptimizationInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, TargetKw: 2}}
	harness.activities.Dispatcher.Snapshots = snapshotter
	harness.activities.Dispatcher.Safety = &replacementSafety{}
	first := &gridosv1.DispatchPlan{EventId: harness.input.EventID, PlanVersion: 2, PlanId: "replacement-2", SolverVersion: "fallback", ModelVersion: "1", DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-3", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, SetpointKw: 1}}}}, Shortfalls: []*gridosv1.ShortfallReport{{IntervalBeginTime: harness.input.Request.BeginTime, IntervalEndTime: harness.input.Request.EndTime, RequestedKw: 1, FeasibleKw: 1}}}
	harness.activities.Dispatcher.Optimizer = activityOptimizer{replacementPlan: first}
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2", "device-3", "device-4"}, Generation: 2}))
	_, updated, err := harness.events.LoadPlan(context.Background(), harness.input.EventID, 2)
	require.NoError(t, err)
	require.Len(t, updated.GetDeviceSchedules(), 2)
	require.Equal(t, 2.0, updated.GetShortfalls()[0].GetRequestedKw())
	require.Equal(t, 2.0, updated.GetShortfalls()[0].GetFeasibleKw())
	snapshotter.snapshot.Optimization.PlanVersion = 3
	harness.activities.Dispatcher.Snapshots = snapshotter
	second := &gridosv1.DispatchPlan{EventId: harness.input.EventID, PlanVersion: 3, PlanId: "replacement-3", SolverVersion: "fallback", ModelVersion: "1", DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-4", Intervals: []*gridosv1.DeviceScheduleInterval{{BeginTime: harness.input.Request.BeginTime, EndTime: harness.input.Request.EndTime, SetpointKw: 1}}}}, Shortfalls: []*gridosv1.ShortfallReport{{IntervalBeginTime: harness.input.Request.BeginTime, IntervalEndTime: harness.input.Request.EndTime, RequestedKw: 1, FeasibleKw: 1}}}
	harness.activities.Dispatcher.Optimizer = activityOptimizer{replacementPlan: second}
	require.NoError(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{EventID: harness.input.EventID, Request: harness.input.Request, DroppedDeviceIDs: []string{"device-2"}, EnvelopeDeviceIDs: []string{"device-2", "device-3", "device-4"}, Generation: 3}))
}
