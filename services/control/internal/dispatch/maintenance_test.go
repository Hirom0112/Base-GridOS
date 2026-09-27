package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestTelemetryMaintenanceRunsOneBoundedPruneActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	environment.RegisterActivityWithOptions(func(context.Context) error { return nil }, activity.RegisterOptions{Name: PruneTelemetryActivity})
	environment.OnActivity(PruneTelemetryActivity, mock.Anything).Return(nil).Once()
	environment.ExecuteWorkflow(TelemetryMaintenance)
	require.True(t, environment.IsWorkflowCompleted())
	require.NoError(t, environment.GetWorkflowError())
	environment.AssertExpectations(t)
}

func TestTelemetryMaintenanceSurfacesPruneFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	environment.RegisterActivityWithOptions(func(context.Context) error { return nil }, activity.RegisterOptions{Name: PruneTelemetryActivity})
	environment.OnActivity(PruneTelemetryActivity, mock.Anything).Return(errors.New("prune failed")).Once()
	environment.ExecuteWorkflow(TelemetryMaintenance)
	require.ErrorContains(t, environment.GetWorkflowError(), "prune failed")
	environment.AssertExpectations(t)
}
