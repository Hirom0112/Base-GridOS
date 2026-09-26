package failures

import (
	"context"
	"errors"
	"sync"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

type Network struct {
	mutex     sync.RWMutex
	connected bool
	publisher telemetry.Publisher
}

func NewNetwork(publisher telemetry.Publisher) (*Network, error) {
	if publisher == nil {
		return nil, errors.New("publisher is required")
	}
	return &Network{connected: true, publisher: publisher}, nil
}

func (network *Network) Disconnect() {
	network.mutex.Lock()
	defer network.mutex.Unlock()
	network.connected = false
}

func (network *Network) Publish(ctx context.Context, observation *gridosv1.TelemetryObservation) error {
	network.mutex.RLock()
	connected := network.connected
	network.mutex.RUnlock()
	if !connected {
		return errors.New("network unavailable")
	}
	return network.publisher.Publish(ctx, observation)
}

func (network *Network) Restore(ctx context.Context, producer *telemetry.Producer) error {
	if producer == nil {
		return errors.New("producer is required")
	}
	network.mutex.Lock()
	network.connected = true
	network.mutex.Unlock()
	return producer.Flush(ctx, network)
}
