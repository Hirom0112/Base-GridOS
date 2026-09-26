package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

type boundedOptimizer struct {
	plan *gridosv1.DispatchPlan
}

type timeoutOptimizer struct {
	activityOptimizer
}

type optimizeTimeoutOptimizer struct {
	activityOptimizer
}

func (optimizeTimeoutOptimizer) Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	return nil, context.DeadlineExceeded
}

func (timeoutOptimizer) Forecast(context.Context, *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error) {
	return nil, context.DeadlineExceeded
}

func (optimizer boundedOptimizer) Optimize(ctx context.Context, request *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > request.GetBudget().AsDuration()+time.Second {
		return nil, errors.New("planning budget was not applied")
	}
	return optimizer.plan, nil
}

func (boundedOptimizer) Forecast(context.Context, *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error) {
	return &gridosv1.ForecastResponse{}, nil
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

func TestPlanningActivitiesFreezeForecastBeforePlanning(t *testing.T) {
	harness := newActivityHarness(t)
	frozen := harness.freeze(t)
	stored, err := storage.NewPostgresEventStore(harness.pool).LoadFrozen(context.Background(), harness.input.EventID, frozen.InputSnapshotID, frozen.EligibilitySnapshotID)
	require.NoError(t, err)
	require.NotNil(t, stored.GetForecast())
	planned, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	require.NoError(t, harness.activities.ValidatePlan(context.Background(), planned))
	require.Equal(t, "VALIDATED", harness.state(t))
}

func TestPlanningActivitiesRecordForecastTransportTimeout(t *testing.T) {
	harness := newActivityHarness(t)
	harness.activities.Dispatcher.Optimizer = timeoutOptimizer{harness.activities.Dispatcher.Optimizer.(activityOptimizer)}
	frozen := harness.freeze(t)
	stored, err := storage.NewPostgresEventStore(harness.pool).LoadFrozen(context.Background(), harness.input.EventID, frozen.InputSnapshotID, frozen.EligibilitySnapshotID)
	require.NoError(t, err)
	require.Contains(t, stored.GetForecast().GetUnavailableSources(), "forecast_transport_timeout")
	var reason string
	err = harness.pool.QueryRow(context.Background(), `SELECT new_values->>'reason' FROM audit_journal WHERE resource_id = $1 AND action = 'FORECAST_TIMEOUT'`, harness.input.EventID).Scan(&reason)
	require.NoError(t, err)
	require.Equal(t, "TRANSPORT_TIMEOUT", reason)
	planned, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	require.NoError(t, harness.activities.ValidatePlan(context.Background(), planned))
	require.Equal(t, "VALIDATED", harness.state(t))
}

func TestPlanningActivitiesRecordOptimizeTransportTimeout(t *testing.T) {
	harness := newActivityHarness(t)
	harness.activities.Dispatcher.Optimizer = optimizeTimeoutOptimizer{harness.activities.Dispatcher.Optimizer.(activityOptimizer)}
	frozen := harness.freeze(t)
	_, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var reason string
	err = harness.pool.QueryRow(context.Background(), `SELECT new_values->>'reason' FROM audit_journal WHERE resource_id = $1 AND action = 'OPTIMIZATION_TIMEOUT'`, harness.input.EventID).Scan(&reason)
	require.NoError(t, err)
	require.Equal(t, "TRANSPORT_TIMEOUT", reason)
	require.Equal(t, "REQUESTED", harness.state(t))
}

func TestPlanningActivitiesRetryUsesStoredPlan(t *testing.T) {
	harness := newActivityHarness(t)
	frozen := harness.freeze(t)
	first, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	second, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	require.Equal(t, first.Input.ApprovalDigest, second.Input.ApprovalDigest)
	require.Equal(t, 1, harness.count(t, "plan_versions"))
}
