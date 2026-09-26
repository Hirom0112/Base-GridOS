package dispatch

import (
	"context"
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
	mockLifecycle(environment, input, PublishCommands)
	attempts := make([]string, 0, 2)
	environment.OnActivity(PublishCommands, mock.Anything, input).Return(temporal.NewApplicationError("unavailable", GatewayTransientError)).Run(func(arguments mock.Arguments) {
		attempts = append(attempts, arguments.Get(1).(Input).CommandID)
	}).Once()
	environment.OnActivity(PublishCommands, mock.Anything, input).Return(nil).Run(func(arguments mock.Arguments) {
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
	mockLifecycle(environment, input, ValidatePlan)
	environment.OnActivity(ValidatePlan, mock.Anything, input).Return(temporal.NewApplicationError("reserve", ValidationError)).Once()

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
	environment.OnActivity(ExpireCommands, mock.Anything, EmergencyCommand{
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
	mockLifecycle(environment, input, TrackAcknowledgements)
	environment.OnActivity(TrackAcknowledgements, mock.Anything, input).Return(nil).Run(func(mock.Arguments) {
		environment.SignalWorkflow(ReplaceDeviceSignal, Replacement{DeviceID: "device-2"})
	}).Once()
	environment.OnActivity(IssueReplacement, mock.Anything, ReplacementCommand{
		EventID: "event-1", DeviceID: "device-2", Generation: 8,
	}).Return(nil).Once()
	approve(environment)

	environment.ExecuteWorkflow(Workflow, input)

	require.NoError(t, environment.GetWorkflowError())
	environment.AssertExpectations(t)
}

func mockLifecycle(environment *testsuite.TestWorkflowEnvironment, input Input, excluded ...func(context.Context, Input) error) {
	skipped := make(map[string]bool, len(excluded))
	for _, activity := range excluded {
		skipped[activityName(activity)] = true
	}
	for _, activity := range []func(context.Context, Input) error{FreezeInputs, RequestPlan, ValidatePlan, PersistIntents, PublishCommands, TrackAcknowledgements, VerifyDelivery, EndEvent, ReconcileLateMessages, ProduceReport} {
		if !skipped[activityName(activity)] {
			environment.OnActivity(activity, mock.Anything, input).Return(nil)
		}
	}
}

func approve(environment *testsuite.TestWorkflowEnvironment) {
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
	}, time.Millisecond)
}
