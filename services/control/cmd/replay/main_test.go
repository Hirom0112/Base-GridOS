package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
	"github.com/stretchr/testify/require"
)

type replaySource struct {
	request *gridosv1.OptimizationRequest
	plan    *gridosv1.DispatchPlan
}

func (source replaySource) Load(context.Context, replay.Manifest) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return source.request, source.plan, nil
}

type replayOptimizer struct {
	plan *gridosv1.DispatchPlan
}

func (optimizer replayOptimizer) Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	return optimizer.plan, nil
}

func TestReplayCLIPrintsIdentical(t *testing.T) {
	directory := t.TempDir()
	fleet := filepath.Join(directory, "fleet.jsonl")
	require.NoError(t, os.WriteFile(fleet, []byte("fleet-v1"), 0o600))
	_, err := replay.Create(directory, replay.Input{
		EventID: "event-1", Seed: 42, FleetFile: fleet,
		InputSnapshotID: "input-1", EligibilitySnapshotID: "eligibility-1", PolicyVersion: "policy-1",
		SolverVersion: "highs", FallbackVersion: "fallback-1", CodeVersion: "abc123",
	})
	require.NoError(t, err)
	request := &gridosv1.OptimizationRequest{EventId: "event-1", PlanVersion: 1, ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-1"}}
	plan := &gridosv1.DispatchPlan{EventId: "event-1", PlanVersion: 1}
	var output bytes.Buffer
	err = run(context.Background(), "event-1", directory, replaySource{request, plan}, replayOptimizer{plan}, &output)
	require.NoError(t, err)
	require.Equal(t, "IDENTICAL\n", output.String())
}
