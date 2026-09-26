package dispatch

import (
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/reconciliation"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue                     = "gridos-dispatch"
	ApproveEventSignal            = "approve-event"
	LaunchEventSignal             = "launch-event"
	EmergencyStopSignal           = "emergency-stop"
	ReplaceDeviceSignal           = "replace-device"
	GatewayTransientError         = "GatewayTransient"
	ValidationError               = "Validation"
	FreezeInputsActivity          = "FreezeInputs"
	RequestPlanActivity           = "RequestPlan"
	ValidatePlanActivity          = "ValidatePlan"
	PersistIntentsActivity        = "PersistIntents"
	PublishCommandsActivity       = "PublishCommands"
	TrackAcknowledgementsActivity = "TrackAcknowledgements"
	VerifyDeliveryActivity        = "VerifyDelivery"
	EndEventActivity              = "EndEvent"
	ReconcileLateMessagesActivity = "ReconcileLateMessages"
	ProduceReportActivity         = "ProduceReport"
	IssueEmergencyStopActivity    = "IssueEmergencyStop"
	ExpireCommandsActivity        = "ExpireCommands"
	IssueReplacementActivity      = "IssueReplacement"
)

type State string

const (
	Requested               State = "REQUESTED"
	Planned                 State = "PLANNED"
	Validated               State = "VALIDATED"
	Approved                State = "APPROVED"
	CommandsPersisted       State = "COMMANDS_PERSISTED"
	Sent                    State = "SENT"
	AcknowledgedOrUncertain State = "ACKNOWLEDGED_OR_UNCERTAIN"
	Executing               State = "EXECUTING"
	Verified                State = "VERIFIED"
	Reconciled              State = "RECONCILED"
	Reported                State = "REPORTED"
)

type Input struct {
	EventID                 string
	CommandID               string
	Generation              uint64
	ExpiresAt               time.Time
	AcknowledgementDeadline time.Time
	PlanVersion             uint64
	Request                 *gridosv1.EventRequest
}

type PersistInput struct {
	Input  Input
	Launch *gridosv1.LaunchEventRequest
}

type Approval struct {
	ApprovedBy string
}

type EmergencyStop struct {
	RequestedBy string
}

type EmergencyCommand struct {
	EventID    string
	Generation uint64
	SetpointKW float64
}

type Replacement struct {
	DeviceID string
}

type ReplacementCommand struct {
	EventID    string
	DeviceID   string
	Generation uint64
}

type Result struct {
	States []State
}

func Workflow(ctx workflow.Context, input Input) (Result, error) {
	if input.EventID == "" {
		return Result{}, errors.New("event ID required")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2,
			MaximumInterval:        10 * time.Second,
			MaximumAttempts:        3,
			NonRetryableErrorTypes: []string{ValidationError},
		},
	})
	result := Result{States: []State{Requested}}
	var frozen FrozenEvent
	if err := workflow.ExecuteActivity(ctx, FreezeInputsActivity, input).Get(ctx, &frozen); err != nil {
		return result, err
	}
	if err := workflow.ExecuteActivity(ctx, RequestPlanActivity, frozen).Get(ctx, &frozen); err != nil {
		return result, err
	}
	input = frozen.Input
	result.States = append(result.States, Planned)
	if err := workflow.ExecuteActivity(ctx, ValidatePlanActivity, frozen).Get(ctx, nil); err != nil {
		return result, err
	}
	result.States = append(result.States, Validated)
	var approval Approval
	workflow.GetSignalChannel(ctx, ApproveEventSignal).Receive(ctx, &approval)
	if approval.ApprovedBy == "" {
		return result, errors.New("approver required")
	}
	result.States = append(result.States, Approved)
	if err := persist(ctx, input); err != nil {
		return result, err
	}
	result.States = append(result.States, CommandsPersisted)
	if err := advance(ctx, PublishCommandsActivity, input, Sent, &result); err != nil {
		return result, err
	}
	emergency := workflow.GetSignalChannel(ctx, EmergencyStopSignal)
	replacements := workflow.GetSignalChannel(ctx, ReplaceDeviceSignal)
	nextGeneration := input.Generation + 1
	if err := waitForAcknowledgements(ctx, input.AcknowledgementDeadline); err != nil {
		return result, err
	}
	if err := advance(ctx, TrackAcknowledgementsActivity, input, AcknowledgedOrUncertain, &result); err != nil {
		return result, err
	}
	if usesEventWindow(ctx, input) {
		if err := runWindow(ctx, input, emergency, replacements, &nextGeneration, &result); err != nil {
			return result, err
		}
		return result, nil
	}
	steps := []struct {
		activity string
		states   []State
	}{
		{VerifyDeliveryActivity, []State{Executing, Verified}},
		{EndEventActivity, nil},
		{ReconcileLateMessagesActivity, []State{Reconciled}},
		{ProduceReportActivity, []State{Reported}},
	}
	for _, step := range steps {
		if err := run(ctx, step.activity, input); err != nil {
			return result, err
		}
		result.States = append(result.States, step.states...)
		if err := handleControlSignals(ctx, emergency, replacements, input.EventID, &nextGeneration); err != nil {
			return result, err
		}
	}
	if err := expire(ctx, input, nextGeneration); err != nil {
		return result, err
	}
	return result, nil
}

func usesEventWindow(ctx workflow.Context, input Input) bool {
	return workflow.GetVersion(ctx, "event-window", workflow.DefaultVersion, 1) != workflow.DefaultVersion && input.Request != nil && input.Request.GetBeginTime() != nil && input.Request.GetEndTime() != nil
}

func runWindow(ctx workflow.Context, input Input, emergency, replacements workflow.ReceiveChannel, generation *uint64, result *Result) error {
	begin := input.Request.GetBeginTime().AsTime()
	end := input.Request.GetEndTime().AsTime()
	if !end.After(begin) {
		return errors.New("event window end must follow begin")
	}
	if err := waitWithControl(ctx, begin, emergency, replacements, input.EventID, generation); err != nil {
		return err
	}
	if err := run(ctx, VerifyDeliveryActivity, input); err != nil {
		return err
	}
	result.States = append(result.States, Executing)
	for intervalEnd := begin.Add(reconciliation.ReportingInterval); ; intervalEnd = intervalEnd.Add(reconciliation.ReportingInterval) {
		if intervalEnd.After(end) {
			intervalEnd = end
		}
		if err := waitWithControl(ctx, intervalEnd, emergency, replacements, input.EventID, generation); err != nil {
			return err
		}
		if err := run(ctx, VerifyDeliveryActivity, input); err != nil {
			return err
		}
		if err := handleControlSignals(ctx, emergency, replacements, input.EventID, generation); err != nil {
			return err
		}
		if !intervalEnd.Before(end) {
			break
		}
	}
	result.States = append(result.States, Verified)
	if err := run(ctx, EndEventActivity, input); err != nil {
		return err
	}
	if err := waitWithControl(ctx, end.Add(30*time.Second), emergency, replacements, input.EventID, generation); err != nil {
		return err
	}
	if err := advance(ctx, ReconcileLateMessagesActivity, input, Reconciled, result); err != nil {
		return err
	}
	if err := advance(ctx, ProduceReportActivity, input, Reported, result); err != nil {
		return err
	}
	return expire(ctx, input, *generation)
}

func waitWithControl(ctx workflow.Context, until time.Time, emergency, replacements workflow.ReceiveChannel, eventID string, generation *uint64) error {
	for workflow.Now(ctx).Before(until) {
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(workflow.NewTimer(ctx, until.Sub(workflow.Now(ctx))), func(workflow.Future) {})
		var stop EmergencyStop
		var replacement Replacement
		var stopped, replaced bool
		selector.AddReceive(emergency, func(channel workflow.ReceiveChannel, _ bool) {
			channel.Receive(ctx, &stop)
			stopped = true
		})
		selector.AddReceive(replacements, func(channel workflow.ReceiveChannel, _ bool) {
			channel.Receive(ctx, &replacement)
			replaced = true
		})
		selector.Select(ctx)
		if stopped {
			if stop.RequestedBy == "" {
				return errors.New("emergency stop requester required")
			}
			if err := workflow.ExecuteActivity(ctx, IssueEmergencyStopActivity, EmergencyCommand{EventID: eventID, Generation: *generation, SetpointKW: 0}).Get(ctx, nil); err != nil {
				return err
			}
			*generation++
		}
		if replaced {
			if replacement.DeviceID == "" {
				return errors.New("replacement device required")
			}
			if err := workflow.ExecuteActivity(ctx, IssueReplacementActivity, ReplacementCommand{EventID: eventID, DeviceID: replacement.DeviceID, Generation: *generation}).Get(ctx, nil); err != nil {
				return err
			}
			*generation++
		}
	}
	return handleControlSignals(ctx, emergency, replacements, eventID, generation)
}

func persist(ctx workflow.Context, input Input) error {
	if workflow.GetVersion(ctx, "launch-signal", workflow.DefaultVersion, 1) == workflow.DefaultVersion {
		return run(ctx, PersistIntentsActivity, input)
	}
	var launch gridosv1.LaunchEventRequest
	workflow.GetSignalChannel(ctx, LaunchEventSignal).Receive(ctx, &launch)
	if launch.GetRequestedBy() == "" || launch.GetPlanVersion() != input.PlanVersion {
		return errors.New("matching launch required")
	}
	return workflow.ExecuteActivity(ctx, PersistIntentsActivity, PersistInput{Input: input, Launch: &launch}).Get(ctx, nil)
}

func waitForAcknowledgements(ctx workflow.Context, deadline time.Time) error {
	if deadline.IsZero() {
		return nil
	}
	return waitUntil(ctx, deadline)
}

func expire(ctx workflow.Context, input Input, generation uint64) error {
	if input.ExpiresAt.IsZero() {
		return nil
	}
	if err := waitUntil(ctx, input.ExpiresAt); err != nil {
		return err
	}
	command := EmergencyCommand{EventID: input.EventID, Generation: generation, SetpointKW: 0}
	return workflow.ExecuteActivity(ctx, ExpireCommandsActivity, command).Get(ctx, nil)
}

func waitUntil(ctx workflow.Context, expiresAt time.Time) error {
	remaining := expiresAt.Sub(workflow.Now(ctx))
	if remaining <= 0 {
		return nil
	}
	return workflow.NewTimer(ctx, remaining).Get(ctx, nil)
}

func advance(ctx workflow.Context, activity string, input Input, state State, result *Result) error {
	if err := run(ctx, activity, input); err != nil {
		return err
	}
	result.States = append(result.States, state)
	return nil
}

func run(ctx workflow.Context, activity string, input Input) error {
	return workflow.ExecuteActivity(ctx, activity, input).Get(ctx, nil)
}

func handleControlSignals(ctx workflow.Context, emergency, replacements workflow.ReceiveChannel, eventID string, generation *uint64) error {
	var stop EmergencyStop
	if emergency.ReceiveAsync(&stop) {
		if stop.RequestedBy == "" {
			return errors.New("emergency stop requester required")
		}
		command := EmergencyCommand{EventID: eventID, Generation: *generation, SetpointKW: 0}
		if err := workflow.ExecuteActivity(ctx, IssueEmergencyStopActivity, command).Get(ctx, nil); err != nil {
			return err
		}
		*generation++
	}
	var replacement Replacement
	if replacements.ReceiveAsync(&replacement) {
		if replacement.DeviceID == "" {
			return errors.New("replacement device required")
		}
		command := ReplacementCommand{EventID: eventID, DeviceID: replacement.DeviceID, Generation: *generation}
		if err := workflow.ExecuteActivity(ctx, IssueReplacementActivity, command).Get(ctx, nil); err != nil {
			return err
		}
		*generation++
	}
	return nil
}
