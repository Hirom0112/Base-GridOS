package dispatch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestSmoke(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	input := Input{EventID: "event-1"}
	mockWorkflowActivities(environment, input)
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
	}, time.Hour)
	environment.ExecuteWorkflow(Workflow, input)

	require.True(t, environment.IsWorkflowCompleted())
	require.NoError(t, environment.GetWorkflowError())
	require.Equal(t, "gridos-dispatch", TaskQueue)
}

func TestLifecycle(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	registerActivities(environment)
	input := Input{EventID: "event-1", Generation: 7}
	frozen := FrozenEvent{Input: input}
	environment.OnActivity(FreezeInputsActivity, mock.Anything, input).Return(frozen, nil).Once()
	environment.OnActivity(RequestPlanActivity, mock.Anything, frozen).Return(frozen, nil).Once()
	environment.OnActivity(ValidatePlanActivity, mock.Anything, frozen).Return(nil).Once()
	activities := []string{PersistIntentsActivity, PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity}
	approved := false
	for _, activity := range activities {
		environment.OnActivity(activity, mock.Anything, input).Return(nil).Run(func(mock.Arguments) {
			if activity == PersistIntentsActivity {
				require.True(t, approved)
			}
		})
	}
	environment.RegisterDelayedCallback(func() {
		approved = true
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
	}, time.Hour)

	environment.ExecuteWorkflow(Workflow, input)

	var result Result
	require.NoError(t, environment.GetWorkflowResult(&result))
	require.Equal(t, []State{
		Requested,
		Planned,
		Validated,
		Approved,
		CommandsPersisted,
		Sent,
		AcknowledgedOrUncertain,
		Executing,
		Verified,
		Reconciled,
		Reported,
	}, result.States)
	environment.AssertExpectations(t)
}

func TestEmergencyStopAfterSent(t *testing.T) {
	for _, trigger := range []string{TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity} {
		t.Run(trigger, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			environment := suite.NewTestWorkflowEnvironment()
			registerActivities(environment)
			input := Input{EventID: "event-1", Generation: 7}
			frozen := FrozenEvent{Input: input}
			environment.OnActivity(FreezeInputsActivity, mock.Anything, input).Return(frozen, nil)
			environment.OnActivity(RequestPlanActivity, mock.Anything, frozen).Return(frozen, nil)
			environment.OnActivity(ValidatePlanActivity, mock.Anything, frozen).Return(nil)
			for _, activity := range []string{PersistIntentsActivity, PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity} {
				call := environment.OnActivity(activity, mock.Anything, input).Return(nil)
				if activity == trigger {
					call.Run(func(mock.Arguments) {
						environment.SignalWorkflow(EmergencyStopSignal, EmergencyStop{RequestedBy: "operator-1"})
					})
				}
			}
			environment.OnActivity(IssueEmergencyStopActivity, mock.Anything, EmergencyCommand{
				EventID: "event-1", Generation: 8, SetpointKW: 0,
			}).Return(nil).Once()
			environment.RegisterDelayedCallback(func() {
				environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
			}, time.Millisecond)

			environment.ExecuteWorkflow(Workflow, input)

			require.NoError(t, environment.GetWorkflowError())
			environment.AssertExpectations(t)
		})
	}
}

func mockWorkflowActivities(environment *testsuite.TestWorkflowEnvironment, input Input) {
	registerActivities(environment)
	frozen := FrozenEvent{Input: input}
	environment.OnActivity(FreezeInputsActivity, mock.Anything, input).Return(frozen, nil)
	environment.OnActivity(RequestPlanActivity, mock.Anything, frozen).Return(frozen, nil)
	environment.OnActivity(ValidatePlanActivity, mock.Anything, frozen).Return(nil)
	for _, activity := range []string{PersistIntentsActivity, PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity} {
		environment.OnActivity(activity, mock.Anything, input).Return(nil)
	}
}

func registerActivities(environment *testsuite.TestWorkflowEnvironment) {
	environment.RegisterActivity(&Activities{})
}
