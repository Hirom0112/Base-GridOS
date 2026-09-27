package protocol

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"math"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CommandHandler struct {
	store              *gateway.Store
	gatewayID          string
	authorizationToken string
	now                func() time.Time
}

func NewCommandHandler(store *gateway.Store, gatewayID, authorizationToken string, now func() time.Time) *CommandHandler {
	return &CommandHandler{store: store, gatewayID: gatewayID, authorizationToken: authorizationToken, now: now}
}

func (handler *CommandHandler) SubmitCommand(ctx context.Context, request *connect.Request[gridosv1.SubmitCommandRequest]) (*connect.Response[gridosv1.SubmitCommandResponse], error) {
	if !authorized(request.Header().Get("Authorization"), handler.authorizationToken) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authorization required"))
	}
	intent := request.Msg.GetCommandIntent()
	command, err := commandFromIntent(intent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	now := handler.now()
	acknowledgement, err := handler.store.AcceptCommand(ctx, command, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	status := gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_REJECTED
	if acknowledgement.Accepted {
		status = gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED
	}
	response := &gridosv1.SubmitCommandResponse{Acknowledgement: &gridosv1.CommandAcknowledgement{
		AcknowledgementId: intent.GetCommandId() + "-ack",
		CommandId:         intent.GetCommandId(),
		IdempotencyKey:    intent.GetIdempotencyKey(),
		ReceiptStatus:     status,
		ReceivedAt:        timestamppb.New(now),
		GatewayId:         handler.gatewayID,
		RejectionReason:   acknowledgement.RejectionReason,
	}}
	return connect.NewResponse(response), nil
}

func commandFromIntent(intent *gridosv1.CommandIntent) (gateway.Command, error) {
	if intent == nil || intent.GetEffectiveAt() == nil || intent.GetExpiresAt() == nil {
		return gateway.Command{}, errors.New("command and time window are required")
	}
	if err := intent.GetEffectiveAt().CheckValid(); err != nil {
		return gateway.Command{}, fmt.Errorf("effective time: %w", err)
	}
	if err := intent.GetExpiresAt().CheckValid(); err != nil {
		return gateway.Command{}, fmt.Errorf("expiry time: %w", err)
	}
	if math.IsNaN(intent.GetSetpointKw()) || math.IsInf(intent.GetSetpointKw(), 0) {
		return gateway.Command{}, errors.New("setpoint must be finite")
	}
	return gateway.Command{
		CommandID:      intent.GetCommandId(),
		IdempotencyKey: intent.GetIdempotencyKey(),
		DeviceID:       intent.GetDeviceId(),
		Generation:     intent.GetGeneration(),
		SetpointKW:     intent.GetSetpointKw(),
		EffectiveAt:    intent.GetEffectiveAt().AsTime(),
		ExpiresAt:      intent.GetExpiresAt().AsTime(),
	}, nil
}

func authorized(actual, expected string) bool {
	if expected == "" || len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
