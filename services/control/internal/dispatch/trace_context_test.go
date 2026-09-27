package dispatch

import (
	"context"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/stretchr/testify/require"
)

type traceCaptureOptimizer struct {
	activityOptimizer
	identities map[string][2]string
}

func (optimizer *traceCaptureOptimizer) Forecast(ctx context.Context, request *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error) {
	correlationID, workflowID := observability.TraceIDs(ctx)
	optimizer.identities["Forecast"] = [2]string{correlationID, workflowID}
	return optimizer.activityOptimizer.Forecast(ctx, request)
}

func (optimizer *traceCaptureOptimizer) Optimize(ctx context.Context, request *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	correlationID, workflowID := observability.TraceIDs(ctx)
	optimizer.identities["Optimize"] = [2]string{correlationID, workflowID}
	return optimizer.activityOptimizer.Optimize(ctx, request)
}

func (optimizer *traceCaptureOptimizer) Replace(ctx context.Context, request *gridosv1.ReplaceRequest) (*gridosv1.ReplaceResponse, error) {
	correlationID, workflowID := observability.TraceIDs(ctx)
	optimizer.identities["Replace"] = [2]string{correlationID, workflowID}
	return optimizer.activityOptimizer.Replace(ctx, request)
}

func TestPlanningActivitiesSeedDecisionTraceContext(t *testing.T) {
	harness := newActivityHarness(t)
	base := harness.activities.Dispatcher.Optimizer.(activityOptimizer)
	optimizer := &traceCaptureOptimizer{activityOptimizer: base, identities: make(map[string][2]string)}
	harness.activities.Dispatcher.Optimizer = optimizer
	frozen := harness.freeze(t)
	_, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	want := [2]string{"correlation-1", "event-1"}
	require.Equal(t, want, optimizer.identities["Forecast"])
	require.Equal(t, want, optimizer.identities["Optimize"])
}
