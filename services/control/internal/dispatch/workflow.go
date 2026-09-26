package dispatch

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue           = "gridos-dispatch"
	ApproveEventSignal  = "approve-event"
	EmergencyStopSignal = "emergency-stop"
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
	EventID    string
	Generation uint64
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

type Result struct {
	States []State
}

func Workflow(ctx workflow.Context, input Input) (Result, error) {
	if input.EventID == "" {
		return Result{}, errors.New("event ID required")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	result := Result{States: []State{Requested}}
	if err := run(ctx, FreezeInputs, input); err != nil {
		return result, err
	}
	if err := advance(ctx, RequestPlan, input, Planned, &result); err != nil {
		return result, err
	}
	if err := advance(ctx, ValidatePlan, input, Validated, &result); err != nil {
		return result, err
	}
	var approval Approval
	workflow.GetSignalChannel(ctx, ApproveEventSignal).Receive(ctx, &approval)
	if approval.ApprovedBy == "" {
		return result, errors.New("approver required")
	}
	result.States = append(result.States, Approved)
	if err := advance(ctx, PersistIntents, input, CommandsPersisted, &result); err != nil {
		return result, err
	}
	if err := advance(ctx, PublishCommands, input, Sent, &result); err != nil {
		return result, err
	}
	emergency := workflow.GetSignalChannel(ctx, EmergencyStopSignal)
	steps := []struct {
		activity func(context.Context, Input) error
		states   []State
	}{
		{TrackAcknowledgements, []State{AcknowledgedOrUncertain}},
		{VerifyDelivery, []State{Executing, Verified}},
		{EndEvent, nil},
		{ReconcileLateMessages, []State{Reconciled}},
		{ProduceReport, []State{Reported}},
	}
	for _, step := range steps {
		if err := run(ctx, step.activity, input); err != nil {
			return result, err
		}
		result.States = append(result.States, step.states...)
		if err := handleEmergency(ctx, emergency, input); err != nil {
			return result, err
		}
	}
	return result, nil
}

func advance(ctx workflow.Context, activity func(context.Context, Input) error, input Input, state State, result *Result) error {
	if err := run(ctx, activity, input); err != nil {
		return err
	}
	result.States = append(result.States, state)
	return nil
}

func run(ctx workflow.Context, activity func(context.Context, Input) error, input Input) error {
	return workflow.ExecuteActivity(ctx, activity, input).Get(ctx, nil)
}

func handleEmergency(ctx workflow.Context, signal workflow.ReceiveChannel, input Input) error {
	var stop EmergencyStop
	if !signal.ReceiveAsync(&stop) {
		return nil
	}
	if stop.RequestedBy == "" {
		return errors.New("emergency stop requester required")
	}
	command := EmergencyCommand{EventID: input.EventID, Generation: input.Generation + 1, SetpointKW: 0}
	return workflow.ExecuteActivity(ctx, IssueEmergencyStop, command).Get(ctx, nil)
}
