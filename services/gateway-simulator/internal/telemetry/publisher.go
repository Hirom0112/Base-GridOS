package telemetry

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
)

type ConnectPublisher struct {
	client             gridosv1connect.TelemetryServiceClient
	gatewayID          string
	authorizationToken string
}

func NewConnectPublisher(client gridosv1connect.TelemetryServiceClient, gatewayID string, authorizationToken string) *ConnectPublisher {
	return &ConnectPublisher{client: client, gatewayID: gatewayID, authorizationToken: authorizationToken}
}

func (publisher *ConnectPublisher) Publish(ctx context.Context, observation *gridosv1.TelemetryObservation) error {
	return publisher.PublishBatch(ctx, []*gridosv1.TelemetryObservation{observation})
}

func (publisher *ConnectPublisher) PublishBatch(ctx context.Context, observations []*gridosv1.TelemetryObservation) error {
	if len(observations) == 0 {
		return nil
	}
	for _, observation := range observations {
		if observation == nil || observation.GetObservationId() == "" {
			return errors.New("observation identifier is required")
		}
	}
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: publisher.gatewayID, Observations: observations})
	request.Header().Set("Authorization", publisher.authorizationToken)
	response, err := publisher.client.PublishTelemetry(ctx, request)
	if err != nil {
		return err
	}
	if response.Msg.GetDurableReceiptId() == "" || len(response.Msg.GetObservationIds()) != len(observations) {
		return errors.New("telemetry receipt does not acknowledge every observation")
	}
	return nil
}
