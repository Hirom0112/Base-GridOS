package dispatch

import (
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue                     = "gridos-dispatch"
	ApproveEventSignal            = "approve-event"
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
	EventID     string
	CommandID   string
	Generation  uint64
	ExpiresAt   time.Time
	PlanVersion uint64
	Request     *gridosv1.EventRequest
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
	if err := advance(ctx, PersistIntentsActivity, input, CommandsPersisted, &result); err != nil {
		return result, err
	}
	if err := advance(ctx, PublishCommandsActivity, input, Sent, &result); err != nil {
		return result, err
	}
	emergency := workflow.GetSignalChannel(ctx, EmergencyStopSignal)
	replacements := workflow.GetSignalChannel(ctx, ReplaceDeviceSignal)
	nextGeneration := input.Generation + 1
	steps := []struct {
		activity string
		states   []State
	}{
		{TrackAcknowledgementsActivity, []State{AcknowledgedOrUncertain}},
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
