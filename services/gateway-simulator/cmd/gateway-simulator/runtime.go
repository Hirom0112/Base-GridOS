package main

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/failures"
)

type runtimeCommandHandler struct {
	next    gridosv1connect.CommandServiceHandler
	runtime *failures.Runtime
	now     func() time.Time
}

func newRuntimeCommandHandler(next gridosv1connect.CommandServiceHandler, runtime *failures.Runtime, now func() time.Time) *runtimeCommandHandler {
	return &runtimeCommandHandler{next: next, runtime: runtime, now: now}
}

func (handler *runtimeCommandHandler) SubmitCommand(ctx context.Context, request *connect.Request[gridosv1.SubmitCommandRequest]) (*connect.Response[gridosv1.SubmitCommandResponse], error) {
	intent := request.Msg.GetCommandIntent()
	deviceID := intent.GetDeviceId()
	targeted := handler.runtime.TargetCommand(handler.now(), intent.GetEventId(), deviceID, intent.GetSetpointKw())
	if handler.affected(deviceID, targeted, failures.OfflineDevices, failures.PartialRegionOutage) {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("device unavailable"))
	}
	response, err := handler.next.SubmitCommand(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.Msg.GetAcknowledgement().GetReceiptStatus() == gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED && request.Msg.GetCommandIntent().GetEventId() != "" {
		for kind := range handler.runtime.RecordCommand(handler.now(), intent.GetEventId(), deviceID, intent.GetSetpointKw()) {
			if targeted == nil {
				targeted = make(map[failures.Kind]bool)
			}
			targeted[kind] = true
		}
	}
	if handler.affected(deviceID, targeted, failures.DuplicatedMessages) {
		if _, err := handler.next.SubmitCommand(ctx, request); err != nil {
			return nil, err
		}
	}
	if handler.affected(deviceID, targeted, failures.DroppedMessages, failures.DelayedGateway) {
		return nil, connect.NewError(connect.CodeDeadlineExceeded, errors.New("command receipt timed out"))
	}
	return response, nil
}

func (handler *runtimeCommandHandler) affected(deviceID string, targeted map[failures.Kind]bool, kinds ...failures.Kind) bool {
	for _, kind := range kinds {
		if targeted[kind] || handler.runtime.Affects(string(kind), deviceID) {
			return true
		}
	}
	return false
}
