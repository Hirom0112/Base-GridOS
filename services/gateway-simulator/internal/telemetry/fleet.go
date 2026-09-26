package telemetry

import (
	"context"
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
)

type Fleet struct {
	producers []*Producer
	cadence   time.Duration
	publisher Publisher
}

func NewFleet(store *gateway.Store, deviceIDs []string, cadence time.Duration, publisher Publisher) (*Fleet, error) {
	if store == nil || len(deviceIDs) == 0 || cadence <= 0 || publisher == nil {
		return nil, errors.New("store, devices, cadence, and publisher are required")
	}
	seen := make(map[string]struct{}, len(deviceIDs))
	producers := make([]*Producer, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if _, exists := seen[deviceID]; exists {
			return nil, errors.New("device identifiers must be unique")
		}
		seen[deviceID] = struct{}{}
		producer, err := NewProducer(store, deviceID, gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, time.Now)
		if err != nil {
			return nil, err
		}
		producers = append(producers, producer)
	}
	return &Fleet{producers: producers, cadence: cadence, publisher: publisher}, nil
}

func (fleet *Fleet) Emit(ctx context.Context, sourceTime time.Time) error {
	if sourceTime.IsZero() {
		return errors.New("source time is required")
	}
	for _, producer := range fleet.producers {
		_, err := producer.Observe(ctx, Sample{
			SourceTime: sourceTime, ObservationTime: sourceTime, State: battery.OnGrid,
		})
		if err != nil {
			return err
		}
	}
	return fleet.producers[0].Flush(ctx, fleet.publisher)
}

func (fleet *Fleet) Run(ctx context.Context, start time.Time) error {
	if start.IsZero() {
		return errors.New("start time is required")
	}
	sourceTime := start
	for {
		if err := fleet.Emit(ctx, sourceTime); err != nil {
			return err
		}
		timer := time.NewTimer(fleet.cadence)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			sourceTime = sourceTime.Add(fleet.cadence)
		}
	}
}
