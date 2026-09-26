package dispatch

import (
	"context"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestSmoke(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	environment.RegisterActivity(FreezeInputs)
	environment.RegisterActivity(RequestPlan)
	environment.RegisterActivity(ValidatePlan)
	environment.RegisterActivity(PersistIntents)
	environment.RegisterActivity(PublishCommands)
	environment.RegisterActivity(TrackAcknowledgements)
	environment.RegisterActivity(VerifyDelivery)
	environment.RegisterActivity(EndEvent)
	environment.RegisterActivity(ReconcileLateMessages)
	environment.RegisterActivity(ProduceReport)
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
	}, time.Hour)
	environment.ExecuteWorkflow(Workflow, Input{EventID: "event-1"})

	require.True(t, environment.IsWorkflowCompleted())
	require.NoError(t, environment.GetWorkflowError())
	require.Equal(t, "gridos-dispatch", TaskQueue)
}

func TestLifecycle(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	input := Input{EventID: "event-1", Generation: 7}
	activities := []func(context.Context, Input) error{
		FreezeInputs,
		RequestPlan,
		ValidatePlan,
		PersistIntents,
		PublishCommands,
		TrackAcknowledgements,
		VerifyDelivery,
		EndEvent,
		ReconcileLateMessages,
		ProduceReport,
	}
	approved := false
	for _, activity := range activities {
		environment.OnActivity(activity, mock.Anything, input).Return(nil).Run(func(mock.Arguments) {
			if activityName(activity) == activityName(activities[3]) {
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
	for _, trigger := range []func(context.Context, Input) error{TrackAcknowledgements, VerifyDelivery, EndEvent, ReconcileLateMessages, ProduceReport} {
		t.Run(activityName(trigger), func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			environment := suite.NewTestWorkflowEnvironment()
			input := Input{EventID: "event-1", Generation: 7}
			for _, activity := range []func(context.Context, Input) error{FreezeInputs, RequestPlan, ValidatePlan, PersistIntents, PublishCommands, TrackAcknowledgements, VerifyDelivery, EndEvent, ReconcileLateMessages, ProduceReport} {
				call := environment.OnActivity(activity, mock.Anything, input).Return(nil)
				if activityName(activity) == activityName(trigger) {
					call.Run(func(mock.Arguments) {
						environment.SignalWorkflow(EmergencyStopSignal, EmergencyStop{RequestedBy: "operator-1"})
					})
				}
			}
			environment.OnActivity(IssueEmergencyStop, mock.Anything, EmergencyCommand{
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

func activityName(activity func(context.Context, Input) error) string {
	return runtime.FuncForPC(reflect.ValueOf(activity).Pointer()).Name()
}
