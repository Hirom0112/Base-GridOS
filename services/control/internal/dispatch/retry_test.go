package dispatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestRetryTransientGatewaySameCommandID(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	input := Input{EventID: "event-1", CommandID: "command-1", Generation: 7}
	mockLifecycle(environment, input, PublishCommandsActivity)
	attempts := make([]string, 0, 2)
	environment.OnActivity(PublishCommandsActivity, mock.Anything, input).Return(temporal.NewApplicationError("unavailable", GatewayTransientError)).Run(func(arguments mock.Arguments) {
		attempts = append(attempts, arguments.Get(1).(Input).CommandID)
	}).Once()
	environment.OnActivity(PublishCommandsActivity, mock.Anything, input).Return(nil).Run(func(arguments mock.Arguments) {
		attempts = append(attempts, arguments.Get(1).(Input).CommandID)
	}).Once()
	approve(environment)

	environment.ExecuteWorkflow(Workflow, input)

	require.NoError(t, environment.GetWorkflowError())
	require.Equal(t, []string{"command-1", "command-1"}, attempts)
}

func TestRetryValidationFailureDoesNotRetry(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	input := Input{EventID: "event-1"}
	frozen := FrozenEvent{Input: input}
	mockLifecycle(environment, input, ValidatePlanActivity)
	environment.OnActivity(ValidatePlanActivity, mock.Anything, frozen).Return(temporal.NewApplicationError("reserve", ValidationError)).Once()

	environment.ExecuteWorkflow(Workflow, input)

	require.Error(t, environment.GetWorkflowError())
	environment.AssertExpectations(t)
}

func TestRetryCommandExpiryUsesDurableTimer(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	expiresAt := environment.Now().Add(time.Hour)
	input := Input{EventID: "event-1", Generation: 7, ExpiresAt: expiresAt}
	mockLifecycle(environment, input)
	environment.OnActivity(ExpireCommandsActivity, mock.Anything, EmergencyCommand{
		EventID: "event-1", Generation: 8, SetpointKW: 0,
	}).Return(nil).Once()
	approve(environment)

	environment.ExecuteWorkflow(Workflow, input)

	require.NoError(t, environment.GetWorkflowError())
	require.False(t, environment.Now().Before(expiresAt))
}

func TestRetryReplacementUsesNewGeneration(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	input := Input{EventID: "event-1", Generation: 7}
	mockLifecycle(environment, input, TrackAcknowledgementsActivity)
	environment.OnActivity(TrackAcknowledgementsActivity, mock.Anything, input).Return(nil).Run(func(mock.Arguments) {
		environment.SignalWorkflow(ReplaceDeviceSignal, Replacement{DeviceID: "device-2"})
	}).Once()
	environment.OnActivity(IssueReplacementActivity, mock.Anything, ReplacementCommand{
		EventID: "event-1", DeviceID: "device-2", Generation: 8,
	}).Return(nil).Once()
	approve(environment)

	environment.ExecuteWorkflow(Workflow, input)

	require.NoError(t, environment.GetWorkflowError())
	environment.AssertExpectations(t)
}

func mockLifecycle(environment *testsuite.TestWorkflowEnvironment, input Input, excluded ...string) {
	registerActivities(environment)
	skipped := make(map[string]bool, len(excluded))
	for _, activity := range excluded {
		skipped[activity] = true
	}
	frozen := FrozenEvent{Input: input}
	if !skipped[FreezeInputsActivity] {
		environment.OnActivity(FreezeInputsActivity, mock.Anything, input).Return(frozen, nil).Maybe()
	}
	if !skipped[RequestPlanActivity] {
		environment.OnActivity(RequestPlanActivity, mock.Anything, frozen).Return(frozen, nil).Maybe()
	}
	if !skipped[ValidatePlanActivity] {
		environment.OnActivity(ValidatePlanActivity, mock.Anything, frozen).Return(nil).Maybe()
	}
	for _, activity := range []string{PersistIntentsActivity, PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity} {
		if !skipped[activity] {
			environment.OnActivity(activity, mock.Anything, input).Return(nil).Maybe()
		}
	}
}

func approve(environment *testsuite.TestWorkflowEnvironment) {
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
	}, time.Millisecond)
}
