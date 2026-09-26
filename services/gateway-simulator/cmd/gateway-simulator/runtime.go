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
	handler.runtime.Advance(handler.now())
	deviceID := request.Msg.GetCommandIntent().GetDeviceId()
	if handler.affected(deviceID, failures.OfflineDevices, failures.PartialRegionOutage) {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("device unavailable"))
	}
	response, err := handler.next.SubmitCommand(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.Msg.GetAcknowledgement().GetReceiptStatus() == gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED && request.Msg.GetCommandIntent().GetEventId() != "" {
		handler.runtime.RecordCommand(handler.now(), request.Msg.GetCommandIntent().GetEventId(), deviceID)
	}
	if handler.affected(deviceID, failures.DuplicatedMessages) {
		if _, err := handler.next.SubmitCommand(ctx, request); err != nil {
			return nil, err
		}
	}
	if handler.affected(deviceID, failures.DroppedMessages, failures.DelayedGateway) {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("command receipt unavailable"))
	}
	return response, nil
}

func (handler *runtimeCommandHandler) affected(deviceID string, kinds ...failures.Kind) bool {
	for _, kind := range kinds {
		if handler.runtime.Affects(string(kind), deviceID) {
			return true
		}
	}
	return false
}
