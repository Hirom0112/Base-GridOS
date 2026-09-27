package replay

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type recordedSource struct {
	request *gridosv1.OptimizationRequest
	plan    *gridosv1.DispatchPlan
}

func (source recordedSource) Load(context.Context, Manifest) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return source.request, source.plan, nil
}

type replayPlanner struct {
	plan  *gridosv1.DispatchPlan
	calls int
}

func (planner *replayPlanner) Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	planner.calls++
	return planner.plan, nil
}

func TestReplayDiffsFrozenPlanAndRejectsChangedInputs(t *testing.T) {
	directory := t.TempDir()
	fleet := filepath.Join(directory, "fleet.jsonl")
	scenario := filepath.Join(directory, "scenario.yaml")
	require.NoError(t, os.WriteFile(fleet, []byte("fleet-v1"), 0o600))
	require.NoError(t, os.WriteFile(scenario, []byte("scenario-v1"), 0o600))
	manifest, err := Create(directory, Input{
		EventID: "event-1", Seed: 42, FleetFile: fleet, ScenarioFile: scenario,
		InputSnapshotID: "input-1", EligibilitySnapshotID: "eligibility-1",
		PolicyVersion: "policy-1", SolverVersion: "highs", FallbackVersion: "fallback-1", CodeVersion: "abc123",
	})
	require.NoError(t, err)
	request := &gridosv1.OptimizationRequest{EventId: "event-1", PlanVersion: 1, ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"}}
	expected := &gridosv1.DispatchPlan{EventId: "event-1", PlanVersion: 1, SolverVersion: "highs", DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "a"}}}
	planner := &replayPlanner{plan: proto.Clone(expected).(*gridosv1.DispatchPlan)}
	result, err := Run(context.Background(), manifest, recordedSource{request: request, plan: expected}, planner)
	require.NoError(t, err)
	require.Equal(t, "IDENTICAL", result.Status)
	require.Empty(t, result.Differences)
	require.Equal(t, 1, planner.calls)
	planner.plan.DeviceSchedules[0].DeviceId = "b"
	result, err = Run(context.Background(), manifest, recordedSource{request: request, plan: expected}, planner)
	require.NoError(t, err)
	require.Equal(t, "DIFFERENT", result.Status)
	require.Equal(t, "device_schedules", result.Differences[0].Field)
	require.NoError(t, os.WriteFile(fleet, []byte("fleet-v2"), 0o600))
	_, err = Run(context.Background(), manifest, recordedSource{request: request, plan: expected}, planner)
	require.ErrorContains(t, err, "fleet hash")
	require.Equal(t, 2, planner.calls)
}
