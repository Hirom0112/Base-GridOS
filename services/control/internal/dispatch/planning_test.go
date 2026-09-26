package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

type boundedOptimizer struct {
	plan *gridosv1.DispatchPlan
}

func (optimizer boundedOptimizer) Optimize(ctx context.Context, request *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > request.GetBudget().AsDuration() {
		return nil, errors.New("planning budget was not applied")
	}
	return optimizer.plan, nil
}

func TestPlanningActivitiesApplyBudgetAndRecordFallback(t *testing.T) {
	harness := newActivityHarness(t)
	snapshotter := harness.activities.Dispatcher.Snapshots.(activitySnapshotter)
	snapshotter.snapshot.Optimization.Budget = durationpb.New(5 * time.Second)
	plan := harness.activities.Dispatcher.Optimizer.(activityOptimizer).plan
	plan.FallbackUsed = true
	plan.FallbackReason = "SOLVER_TIMEOUT"
	harness.activities.Dispatcher.Optimizer = boundedOptimizer{plan: plan}
	frozen := harness.freeze(t)
	_, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	var reason string
	err = harness.pool.QueryRow(context.Background(), `SELECT new_values->>'fallback_reason' FROM audit_journal WHERE resource_id = $1 AND action = 'PLAN_FALLBACK_SELECTED'`, harness.input.EventID).Scan(&reason)
	require.NoError(t, err)
	require.Equal(t, "SOLVER_TIMEOUT", reason)
}
