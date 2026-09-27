package replay

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	corereplay "github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type replaySource struct {
	expected *gridosv1.DispatchPlan
	updates  []*gridosv1.EventTimelineEntry
}

func (source replaySource) Load(context.Context, corereplay.Manifest) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return &gridosv1.OptimizationRequest{EventId: "event-1", PlanVersion: 1, ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"}}, source.expected, nil
}

func (source replaySource) Timeline(context.Context, string) ([]*gridosv1.EventTimelineEntry, error) {
	return source.updates, nil
}

type replayPlanner struct{ actual *gridosv1.DispatchPlan }

func (planner replayPlanner) Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	return planner.actual, nil
}

func TestReplayEventReturnsManifestTimelineAndDiff(t *testing.T) {
	directory := t.TempDir()
	fleet := filepath.Join(directory, "fleet.jsonl")
	if err := os.WriteFile(fleet, []byte("fleet"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := corereplay.Create(directory, corereplay.Input{EventID: "event-1", Seed: 42, FleetFile: fleet,
		InputSnapshotID: "input-1", EligibilitySnapshotID: "eligible-1", PolicyVersion: "policy-1",
		SolverVersion: "solver-1", FallbackVersion: "fallback-1", CodeVersion: "build-1"})
	if err != nil {
		t.Fatal(err)
	}
	stamp := timestamppb.New(time.Unix(100, 0))
	source := replaySource{expected: &gridosv1.DispatchPlan{EventId: "event-1", PlanVersion: 1}, updates: []*gridosv1.EventTimelineEntry{{Sequence: 1, OccurredAt: stamp, Action: "CREATED"}}}
	service := NewService(directory, source, source, replayPlanner{actual: &gridosv1.DispatchPlan{EventId: "event-1", PlanVersion: 1, DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1"}}}})
	request := connect.NewRequest(&gridosv1.ReplayEventRequest{EventId: "event-1"})
	request.Header().Set("X-GridOS-Role", "analyst")
	response, err := service.ReplayEvent(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	result := response.Msg
	assertReplayProvenance(t, result)
	if len(result.GetUpdates()) != 1 || result.GetUpdates()[0].GetSequence() != 1 || !result.GetUpdates()[0].GetOccurredAt().AsTime().Equal(stamp.AsTime()) {
		t.Fatalf("timeline missing: %+v", result.GetUpdates())
	}
	if result.GetDiffStatus() != "DIFFERENT" || len(result.GetDifferences()) != 1 || result.GetDifferences()[0].GetField() != "device_schedules" {
		t.Fatalf("replay diff missing: %+v", result)
	}
}

func assertReplayProvenance(t *testing.T, result *gridosv1.ReplayEventResponse) {
	t.Helper()
	if result.GetSeed() != 42 || result.GetInputSnapshotId() != "input-1" || result.GetEligibilitySnapshotId() != "eligible-1" ||
		result.GetPolicyVersion() != "policy-1" || result.GetSolverVersion() != "solver-1" || result.GetFallbackVersion() != "fallback-1" ||
		result.GetCodeVersion() != "build-1" || len(result.GetFleetSha256()) != 64 || result.GetScenarioSha256() != "" {
		t.Fatalf("manifest provenance missing: %+v", result)
	}
}

func TestReplayEventRequiresAuthorizedRole(t *testing.T) {
	service := NewService(t.TempDir(), replaySource{}, replaySource{}, replayPlanner{})
	request := connect.NewRequest(&gridosv1.ReplayEventRequest{EventId: "event-1"})
	_, err := service.ReplayEvent(context.Background(), request)
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("missing role: %v", err)
	}
	request.Header().Set("X-GridOS-Role", "viewer")
	_, err = service.ReplayEvent(context.Background(), request)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("unauthorized role: %v", err)
	}
}
