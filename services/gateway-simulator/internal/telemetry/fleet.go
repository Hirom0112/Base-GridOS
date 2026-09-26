package telemetry

import (
	"context"
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"google.golang.org/protobuf/encoding/protojson"
)

type BatchPublisher interface {
	PublishBatch(context.Context, []*gridosv1.TelemetryObservation) error
}

var ErrPublishUnavailable = errors.New("telemetry publisher unavailable")

type Effects interface {
	Advance(time.Time)
	Affects(string, string) bool
}

type Fleet struct {
	producers  []*Producer
	cadence    time.Duration
	publisher  Publisher
	store      *gateway.Store
	effects    Effects
	sourceStep time.Duration
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
	return &Fleet{producers: producers, cadence: cadence, publisher: publisher, store: store, sourceStep: cadence}, nil
}

func (fleet *Fleet) SetEffects(effects Effects) {
	fleet.effects = effects
}

func (fleet *Fleet) SetSourceStep(step time.Duration) error {
	if step <= 0 {
		return errors.New("source step is required")
	}
	fleet.sourceStep = step
	return nil
}

func (fleet *Fleet) Emit(ctx context.Context, sourceTime time.Time) error {
	if sourceTime.IsZero() {
		return errors.New("source time is required")
	}
	if fleet.effects != nil {
		fleet.effects.Advance(sourceTime)
	}
	deferred := make(map[string]bool)
	duplicated := make(map[string]bool)
	for _, producer := range fleet.producers {
		var observation *gridosv1.TelemetryObservation
		var err error
		if fleet.affected("OFFLINE_DEVICES", producer.deviceID) || fleet.affected("PARTIAL_REGION_OUTAGE", producer.deviceID) {
			observation, err = producer.Gap(ctx, sourceTime)
		} else {
			observation, err = producer.Observe(ctx, Sample{SourceTime: sourceTime, ObservationTime: sourceTime, State: battery.OnGrid})
		}
		if err != nil {
			return err
		}
		if fleet.affected("DROPPED_MESSAGES", producer.deviceID) {
			if err := fleet.store.ConfirmObservation(ctx, observation.GetObservationId()); err != nil {
				return err
			}
			continue
		}
		deferred[observation.GetObservationId()] = fleet.affected("DELAYED_TELEMETRY", producer.deviceID) || fleet.affected("DELAYED_GATEWAY", producer.deviceID)
		duplicated[observation.GetObservationId()] = fleet.affected("DUPLICATED_MESSAGES", producer.deviceID)
	}
	batch, ok := fleet.publisher.(BatchPublisher)
	if !ok {
		return fleet.producers[0].Flush(ctx, fleet.publisher)
	}
	return fleet.flushBatch(ctx, batch, deferred, duplicated)
}

func (fleet *Fleet) affected(kind, deviceID string) bool {
	return fleet.effects != nil && fleet.effects.Affects(kind, deviceID)
}

func (fleet *Fleet) flushBatch(ctx context.Context, publisher BatchPublisher, deferred, duplicated map[string]bool) error {
	buffered, err := fleet.store.BufferedObservations(ctx)
	if err != nil {
		return err
	}
	observations := make([]*gridosv1.TelemetryObservation, 0, len(buffered))
	for _, item := range buffered {
		if deferred[item.ObservationID] {
			continue
		}
		observation := &gridosv1.TelemetryObservation{}
		if err := protojson.Unmarshal(item.Payload, observation); err != nil {
			return err
		}
		observations = append(observations, observation)
		if duplicated[item.ObservationID] {
			observations = append(observations, observation)
		}
	}
	if len(observations) == 0 {
		return nil
	}
	if err := publisher.PublishBatch(ctx, observations); err != nil {
		return err
	}
	for _, item := range buffered {
		if deferred[item.ObservationID] {
			continue
		}
		if err := fleet.store.ConfirmObservation(ctx, item.ObservationID); err != nil {
			return err
		}
	}
	return nil
}

func (fleet *Fleet) Run(ctx context.Context, start time.Time) error {
	if start.IsZero() {
		return errors.New("start time is required")
	}
	sourceTime := start
	for {
		if err := fleet.Emit(ctx, sourceTime); err != nil && !errors.Is(err, ErrPublishUnavailable) {
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
			sourceTime = sourceTime.Add(fleet.sourceStep)
		}
	}
}
