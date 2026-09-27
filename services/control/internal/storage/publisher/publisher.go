package publisher

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	Pool                   *pgxpool.Pool
	Client                 gridosv1connect.CommandServiceClient
	AuthorizationToken     string
	BatchSize              int
	LeaseDuration          time.Duration
	AcknowledgementTimeout time.Duration
	Now                    func() time.Time
	Interval               func(storage.ClaimedCommand, time.Time) storage.FeasiblePowerInterval
}

type Publisher struct {
	config Config
}

func New(config Config) *Publisher {
	return &Publisher{config: config}
}

func (publisher *Publisher) PublishBatch(ctx context.Context) error {
	ctx, span := otel.Tracer("gridos.control").Start(ctx, "publisher.batch")
	defer span.End()
	now := publisher.config.Now()
	commands, err := storage.ClaimOutbox(ctx, publisher.config.Pool, storage.OutboxClaim{
		AvailableAt: now,
		LeaseUntil:  now.Add(publisher.config.LeaseDuration),
		BatchSize:   publisher.config.BatchSize,
	})
	if err != nil {
		return err
	}
	for _, command := range commands {
		if err = publisher.publishWithRetry(ctx, command); err != nil {
			return err
		}
	}
	return nil
}

func (publisher *Publisher) publishWithRetry(ctx context.Context, command storage.ClaimedCommand) error {
	for {
		err := publisher.Publish(ctx, command)
		if err == nil || connect.CodeOf(err) != connect.CodeUnavailable {
			return err
		}
		if command.Attempts >= 3 {
			at := publisher.config.Now()
			interval := publisher.config.Interval(command, at)
			_, err = storage.MarkAcknowledgementUncertain(ctx, publisher.config.Pool, at, at, interval)
			return err
		}
		timer := time.NewTimer(time.Duration(command.Attempts) * 50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		command.Attempts, err = storage.MarkOutboxRetry(ctx, publisher.config.Pool, command.CommandID, publisher.config.Now().Add(publisher.config.LeaseDuration))
		if err != nil {
			return err
		}
	}
}

func (publisher *Publisher) Publish(ctx context.Context, command storage.ClaimedCommand) error {
	now := publisher.config.Now()
	_, err := storage.TransitionCommand(ctx, publisher.config.Pool, storage.CommandTransition{
		CommandID:     command.CommandID,
		ExpectedState: "PERSISTED",
		NextState:     "SENT",
		OccurredAt:    now,
		CorrelationID: command.CorrelationID,
	})
	if err != nil {
		return err
	}
	request := connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: commandIntent(command)})
	request.Header().Set("Authorization", publisher.config.AuthorizationToken)
	deliveryContext, cancel := context.WithTimeout(ctx, publisher.config.AcknowledgementTimeout)
	defer cancel()
	started := time.Now()
	response, err := publisher.config.Client.SubmitCommand(deliveryContext, request)
	latency := time.Since(started)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || connect.CodeOf(err) == connect.CodeDeadlineExceeded {
			deadline := now.Add(publisher.config.AcknowledgementTimeout)
			interval := publisher.config.Interval(command, deadline)
			_, markErr := storage.MarkAcknowledgementUncertain(ctx, publisher.config.Pool, deadline, deadline, interval)
			if markErr != nil {
				return errors.Join(context.DeadlineExceeded, markErr)
			}
			return nil
		}
		return err
	}
	acknowledgement, err := acknowledgementFromResponse(command, response.Msg.GetAcknowledgement())
	if err != nil {
		return err
	}
	_ = observability.ProcessMetrics.ObserveAckLatency(latency)
	if err = storage.RecordAcknowledgement(ctx, publisher.config.Pool, acknowledgement); err != nil {
		return err
	}
	return storage.MarkOutboxPublished(ctx, publisher.config.Pool, command.CommandID, acknowledgement.ReceivedAt)
}

func commandIntent(command storage.ClaimedCommand) *gridosv1.CommandIntent {
	return &gridosv1.CommandIntent{
		CommandId:      command.CommandID,
		IdempotencyKey: command.IdempotencyKey,
		DeviceId:       command.DeviceID,
		EventId:        command.EventID,
		PlanVersion:    uint64(command.PlanVersion),
		Generation:     uint64(command.Generation),
		SetpointKw:     command.SetpointKW,
		IssuedAt:       timestamppb.New(command.IssuedAt),
		EffectiveAt:    timestamppb.New(command.EffectiveAt),
		ExpiresAt:      timestamppb.New(command.ExpiresAt),
		PolicyVersion:  command.PolicyVersion,
		CorrelationId:  command.CorrelationID,
	}
}

func acknowledgementFromResponse(command storage.ClaimedCommand, response *gridosv1.CommandAcknowledgement) (storage.Acknowledgement, error) {
	if response == nil || response.GetReceivedAt() == nil {
		return storage.Acknowledgement{}, errors.New("gateway acknowledgement is incomplete")
	}
	if response.GetCommandId() != command.CommandID || response.GetIdempotencyKey() != command.IdempotencyKey {
		return storage.Acknowledgement{}, errors.New("gateway acknowledgement does not match command")
	}
	status := "ACCEPTED"
	if response.GetReceiptStatus() == gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_REJECTED {
		status = "REJECTED"
	} else if response.GetReceiptStatus() != gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED {
		return storage.Acknowledgement{}, errors.New("gateway acknowledgement status is unspecified")
	}
	return storage.Acknowledgement{
		AcknowledgementID: response.GetAcknowledgementId(),
		CommandID:         response.GetCommandId(),
		IdempotencyKey:    response.GetIdempotencyKey(),
		ReceiptStatus:     status,
		ReceivedAt:        response.GetReceivedAt().AsTime(),
		GatewayID:         response.GetGatewayId(),
		RejectionReason:   response.GetRejectionReason(),
		CorrelationID:     command.CorrelationID,
	}, nil
}
