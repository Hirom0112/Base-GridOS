package telemetry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Sample struct {
	SourceTime           time.Time
	ObservationTime      time.Time
	FromGridKW           float64
	FromStorageKW        float64
	FromSolarKW          float64
	NonSolarToHomeKW     float64
	ToHomeKW             float64
	StateOfEnergyPercent float64
	GridVoltageVolts     float64
	State                battery.OperatingState
	BackupHoursCurrent   float64
	BackupHours750W      float64
	OvercurrentLimitKW   float64
}

type Publisher interface {
	Publish(context.Context, *gridosv1.TelemetryObservation) error
}

type Producer struct {
	store    *gateway.Store
	deviceID string
	boundary gridosv1.MeasurementBoundary
	now      func() time.Time
}

func NewProducer(store *gateway.Store, deviceID string, boundary gridosv1.MeasurementBoundary, now func() time.Time) (*Producer, error) {
	if store == nil || deviceID == "" || now == nil {
		return nil, errors.New("store, device identifier, and clock are required")
	}
	if boundary == gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_UNSPECIFIED {
		return nil, errors.New("measurement boundary is required")
	}
	return &Producer{store: store, deviceID: deviceID, boundary: boundary, now: now}, nil
}

func (producer *Producer) Observe(ctx context.Context, sample Sample) (*gridosv1.TelemetryObservation, error) {
	sequence, err := producer.store.NextTelemetrySequence(ctx, producer.deviceID)
	if err != nil {
		return nil, err
	}
	observation, err := producer.sampleObservation(sequence, sample)
	if err != nil {
		return nil, err
	}
	if err := producer.buffer(ctx, observation); err != nil {
		return nil, err
	}
	return observation, nil
}

func (producer *Producer) sampleObservation(sequence uint64, sample Sample) (*gridosv1.TelemetryObservation, error) {
	if err := validateSample(sample); err != nil {
		return nil, err
	}
	observation := producer.baseObservation(sequence, sample.SourceTime, sample.ObservationTime)
	observation.ValueState = gridosv1.ValueState_VALUE_STATE_PRESENT
	observation.PowerFlow = &gridosv1.PowerFlow{
		FromGridKw:       sample.FromGridKW,
		FromStorageKw:    sample.FromStorageKW,
		FromSolarKw:      sample.FromSolarKW,
		NonSolarToHomeKw: sample.NonSolarToHomeKW,
		ToHomeKw:         sample.ToHomeKW,
	}
	observation.StateOfEnergyPercent = sample.StateOfEnergyPercent
	observation.GridVoltageVolts = sample.GridVoltageVolts
	if err := setOperatingState(observation, sample); err != nil {
		return nil, err
	}
	return observation, nil
}

func (producer *Producer) Gap(ctx context.Context, observationTime time.Time) (*gridosv1.TelemetryObservation, error) {
	sequence, err := producer.store.NextTelemetrySequence(ctx, producer.deviceID)
	if err != nil {
		return nil, err
	}
	observation := producer.gapObservation(sequence, observationTime)
	if err := producer.buffer(ctx, observation); err != nil {
		return nil, err
	}
	return observation, nil
}

func (producer *Producer) gapObservation(sequence uint64, observationTime time.Time) *gridosv1.TelemetryObservation {
	observation := producer.baseObservation(sequence, observationTime, observationTime)
	observation.ValueState = gridosv1.ValueState_VALUE_STATE_MISSING
	observation.OperatingState = &gridosv1.TelemetryObservation_TelemetryUnavailable{
		TelemetryUnavailable: &gridosv1.TelemetryUnavailable{ObservedAt: timestamppb.New(observationTime)},
	}
	return observation
}

func (producer *Producer) baseObservation(sequence uint64, sourceTime, observationTime time.Time) *gridosv1.TelemetryObservation {
	receiveTime := producer.now()
	return &gridosv1.TelemetryObservation{
		ObservationId:       fmt.Sprintf("%s-%d", producer.deviceID, sequence),
		DeviceId:            producer.deviceID,
		SourceTime:          timestamppb.New(sourceTime),
		ReceiveTime:         timestamppb.New(receiveTime),
		ObservationTime:     timestamppb.New(observationTime),
		Units:               gridosv1.TelemetryUnits_TELEMETRY_UNITS_KW_PERCENT_VOLTS,
		SignConvention:      gridosv1.SignConvention_SIGN_CONVENTION_AC_SIDE_DISCHARGE_POSITIVE,
		Sequence:            sequence,
		QualityFlags:        []gridosv1.TelemetryQualityFlag{gridosv1.TelemetryQualityFlag_TELEMETRY_QUALITY_FLAG_VALID},
		MeasurementBoundary: producer.boundary,
	}
}

func (producer *Producer) buffer(ctx context.Context, observation *gridosv1.TelemetryObservation) error {
	payload, err := protojson.Marshal(observation)
	if err != nil {
		return err
	}
	return producer.store.BufferObservation(ctx, observation.GetObservationId(), payload)
}

func (producer *Producer) Flush(ctx context.Context, publisher Publisher) error {
	if publisher == nil {
		return errors.New("publisher is required")
	}
	buffered, err := producer.store.BufferedObservations(ctx)
	if err != nil {
		return err
	}
	for _, item := range buffered {
		observation := &gridosv1.TelemetryObservation{}
		if err := protojson.Unmarshal(item.Payload, observation); err != nil {
			return err
		}
		if err := publisher.Publish(ctx, observation); err != nil {
			return err
		}
		if err := producer.store.ConfirmObservation(ctx, item.ObservationID); err != nil {
			return err
		}
	}
	return nil
}

func validateSample(sample Sample) error {
	values := []float64{sample.FromGridKW, sample.FromStorageKW, sample.FromSolarKW, sample.NonSolarToHomeKW, sample.ToHomeKW, sample.StateOfEnergyPercent, sample.GridVoltageVolts, sample.BackupHoursCurrent, sample.BackupHours750W, sample.OvercurrentLimitKW}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("sample values must be finite")
		}
	}
	if sample.SourceTime.IsZero() || sample.ObservationTime.IsZero() {
		return errors.New("sample times are required")
	}
	if math.Abs(sample.ToHomeKW-sample.FromGridKW-sample.FromStorageKW-sample.FromSolarKW) > 0.000001 {
		return errors.New("power flow does not balance")
	}
	if sample.StateOfEnergyPercent < 0 || sample.StateOfEnergyPercent > 100 {
		return errors.New("state of energy is out of range")
	}
	return nil
}

func setOperatingState(observation *gridosv1.TelemetryObservation, sample Sample) error {
	observedAt := timestamppb.New(sample.ObservationTime)
	switch sample.State {
	case battery.OnGrid:
		observation.OperatingState = &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: observedAt, EstimatedBackupHoursAtCurrentUsage: sample.BackupHoursCurrent, EstimatedBackupHoursAt_750Watts: sample.BackupHours750W}}
	case battery.OffGridOutage:
		observation.OperatingState = &gridosv1.TelemetryObservation_OffGridOutage{OffGridOutage: &gridosv1.OffGridOutage{ObservedAt: observedAt, EstimatedBackupHoursAtCurrentUsage: sample.BackupHoursCurrent, EstimatedBackupHoursAt_750Watts: sample.BackupHours750W}}
	case battery.OffGridNoHomePower:
		observation.OperatingState = &gridosv1.TelemetryObservation_OffGridNoHomePower{OffGridNoHomePower: &gridosv1.OffGridNoHomePower{ObservedAt: observedAt, EstimatedBackupHoursAtCurrentUsage: sample.BackupHoursCurrent, EstimatedBackupHoursAt_750Watts: sample.BackupHours750W}}
	case battery.OffGridOvercurrent:
		observation.OperatingState = &gridosv1.TelemetryObservation_OffGridOvercurrent{OffGridOvercurrent: &gridosv1.OffGridOvercurrent{ObservedAt: observedAt, EstimatedBackupHoursAtCurrentUsage: sample.BackupHoursCurrent, EstimatedBackupHoursAt_750Watts: sample.BackupHours750W, OvercurrentLimitKw: sample.OvercurrentLimitKW}}
	case battery.OffGridOvercurrentStandby:
		observation.OperatingState = &gridosv1.TelemetryObservation_OffGridOvercurrentStandby{OffGridOvercurrentStandby: &gridosv1.OffGridOvercurrentStandby{ObservedAt: observedAt, EstimatedBackupHoursAtCurrentUsage: sample.BackupHoursCurrent, EstimatedBackupHoursAt_750Watts: sample.BackupHours750W}}
	case battery.TelemetryUnavailable:
		return errors.New("use Gap for unavailable telemetry")
	default:
		return fmt.Errorf("unknown operating state %q", sample.State)
	}
	return nil
}
