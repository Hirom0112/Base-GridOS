package dispatch

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWindowReportWaitsForEndAndLateMessages(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	begin := environment.Now().Add(10 * time.Minute)
	end := begin.Add(20 * time.Minute)
	input := Input{EventID: "window-event", Request: &gridosv1.EventRequest{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end)}}
	mockWorkflowActivities(environment, input)
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
		environment.SignalWorkflow(LaunchEventSignal, persistArgument(input).Launch)
	}, time.Millisecond)
	environment.ExecuteWorkflow(Workflow, input)
	require.NoError(t, environment.GetWorkflowError())
	require.False(t, environment.Now().Before(end.Add(30*time.Second)))
}

func TestWindowProcessesSignalsDuringMeasurement(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	begin := environment.Now().Add(10 * time.Minute)
	input := Input{EventID: "window-signals", Generation: 4, Request: &gridosv1.EventRequest{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(10 * time.Minute))}}
	mockWorkflowActivities(environment, input)
	environment.OnActivity(IssueEmergencyStopActivity, mock.Anything, EmergencyCommand{EventID: input.EventID, Generation: 5, SetpointKW: 0}).Return(nil).Once()
	environment.OnActivity(IssueReplacementActivity, mock.Anything, ReplacementCommand{EventID: input.EventID, DeviceID: "replacement-1", Generation: 6}).Return(nil).Once()
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
		environment.SignalWorkflow(LaunchEventSignal, persistArgument(input).Launch)
	}, time.Millisecond)
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(EmergencyStopSignal, EmergencyStop{RequestedBy: "operator-1"})
		environment.SignalWorkflow(ReplaceDeviceSignal, Replacement{DeviceID: "replacement-1"})
	}, 15*time.Minute)
	environment.ExecuteWorkflow(Workflow, input)
	require.NoError(t, environment.GetWorkflowError())
	environment.AssertExpectations(t)
}

func TestSmoke(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	input := Input{EventID: "event-1"}
	mockWorkflowActivities(environment, input)
	environment.RegisterDelayedCallback(func() {
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
		environment.SignalWorkflow(LaunchEventSignal, persistArgument(input).Launch)
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
	activities := []string{PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity}
	approved := false
	environment.OnActivity(PersistIntentsActivity, mock.Anything, persistArgument(input)).Return(nil).Run(func(mock.Arguments) {
		require.True(t, approved)
	})
	for _, activity := range activities {
		environment.OnActivity(activity, mock.Anything, input).Return(nil)
	}
	environment.RegisterDelayedCallback(func() {
		approved = true
		environment.SignalWorkflow(ApproveEventSignal, Approval{ApprovedBy: "operator-1"})
		environment.SignalWorkflow(LaunchEventSignal, &gridosv1.LaunchEventRequest{PlanVersion: input.PlanVersion, RequestedBy: "operator-1"})
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
			environment.OnActivity(PersistIntentsActivity, mock.Anything, persistArgument(input)).Return(nil)
			for _, activity := range []string{PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity} {
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
				environment.SignalWorkflow(LaunchEventSignal, &gridosv1.LaunchEventRequest{PlanVersion: input.PlanVersion, RequestedBy: "operator-1"})
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
	environment.OnActivity(PersistIntentsActivity, mock.Anything, persistArgument(input)).Return(nil)
	for _, activity := range []string{PublishCommandsActivity, TrackAcknowledgementsActivity, VerifyDeliveryActivity, EndEventActivity, ReconcileLateMessagesActivity, ProduceReportActivity} {
		environment.OnActivity(activity, mock.Anything, input).Return(nil)
	}
}

func persistArgument(input Input) PersistInput {
	return PersistInput{Input: input, Launch: &gridosv1.LaunchEventRequest{PlanVersion: input.PlanVersion, RequestedBy: "operator-1"}}
}

func registerActivities(environment *testsuite.TestWorkflowEnvironment) {
	environment.RegisterActivity(&Activities{})
	environment.RegisterActivityWithOptions(func(context.Context, Input) error { return nil }, activity.RegisterOptions{Name: VerifyDeliveryActivity})
	environment.RegisterActivityWithOptions(func(context.Context, Input) error { return nil }, activity.RegisterOptions{Name: ReconcileLateMessagesActivity})
}
