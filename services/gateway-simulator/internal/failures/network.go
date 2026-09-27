package failures

import (
	"context"
	"errors"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

type Network struct {
	publisher *telemetry.ConnectPublisher
}

func NewNetwork(publisher *telemetry.ConnectPublisher) (*Network, error) {
	if publisher == nil {
		return nil, errors.New("publisher is required")
	}
	return &Network{publisher: publisher}, nil
}

func (network *Network) Publish(ctx context.Context, observation *gridosv1.TelemetryObservation) error {
	if err := network.publisher.Publish(ctx, observation); err != nil {
		return errors.Join(telemetry.ErrPublishUnavailable, err)
	}
	return nil
}

func (network *Network) PublishBatch(ctx context.Context, observations []*gridosv1.TelemetryObservation) error {
	if err := network.publisher.PublishBatch(ctx, observations); err != nil {
		return errors.Join(telemetry.ErrPublishUnavailable, err)
	}
	return nil
}
