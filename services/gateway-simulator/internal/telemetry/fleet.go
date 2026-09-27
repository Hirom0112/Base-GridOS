package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strconv"
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

type Device struct {
	DeviceID        string
	LoadProfileType string
	SimulationSeed  int64
	Parameters      battery.Parameters
}

type physicalProducer struct {
	producer *Producer
	model    *battery.Model
	device   Device
}

type Fleet struct {
	producers  []physicalProducer
	profiles   Profiles
	cadence    time.Duration
	publisher  Publisher
	store      *gateway.Store
	effects    Effects
	sourceStep time.Duration
	lastSource time.Time
}

func NewFleet(store *gateway.Store, devices []Device, profiles Profiles, cadence time.Duration, publisher Publisher) (*Fleet, error) {
	if store == nil || len(devices) == 0 || len(profiles) == 0 || cadence <= 0 || publisher == nil {
		return nil, errors.New("store, devices, cadence, and publisher are required")
	}
	seen := make(map[string]struct{}, len(devices))
	producers := make([]physicalProducer, 0, len(devices))
	for _, device := range devices {
		if device.DeviceID == "" || device.LoadProfileType == "" {
			return nil, errors.New("device and load profile identifiers are required")
		}
		if _, exists := seen[device.DeviceID]; exists {
			return nil, errors.New("device identifiers must be unique")
		}
		if _, exists := profiles[device.LoadProfileType]; !exists {
			return nil, errors.New("device load profile is unavailable")
		}
		seen[device.DeviceID] = struct{}{}
		producer, err := NewProducer(store, device.DeviceID, gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, time.Now)
		if err != nil {
			return nil, err
		}
		model, err := battery.New(device.Parameters, seededEnergy(device), battery.OnGrid)
		if err != nil {
			return nil, err
		}
		producers = append(producers, physicalProducer{producer: producer, model: model, device: device})
	}
	return &Fleet{producers: producers, profiles: profiles, cadence: cadence, publisher: publisher, store: store, sourceStep: cadence}, nil
}

func seededEnergy(device Device) float64 {
	digest := sha256.Sum256([]byte(strconv.FormatInt(device.SimulationSeed, 10) + ":" + device.DeviceID))
	unit := float64(binary.BigEndian.Uint64(digest[:8])>>11) / float64(uint64(1)<<53)
	floor := device.Parameters.HardwareFloorKWh
	return floor + (device.Parameters.UsableEnergyKWh-floor)*(0.6+0.35*unit)
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
	duration := fleet.sourceStep
	if !fleet.lastSource.IsZero() {
		duration = sourceTime.Sub(fleet.lastSource)
	}
	if duration <= 0 {
		return errors.New("source time must advance")
	}
	if fleet.effects != nil {
		fleet.effects.Advance(sourceTime)
	}
	active, err := fleet.activeCommands(ctx, sourceTime)
	if err != nil {
		return err
	}
	deferred := make(map[string]bool)
	duplicated := make(map[string]bool)
	observations, err := fleet.bufferPhysical(ctx, active, sourceTime, duration)
	if err != nil {
		return err
	}
	for _, observation := range observations {
		deviceID := observation.GetDeviceId()
		if fleet.affected("DROPPED_MESSAGES", deviceID) {
			if err := fleet.store.ConfirmObservation(ctx, observation.GetObservationId()); err != nil {
				return err
			}
			continue
		}
		deferred[observation.GetObservationId()] = fleet.affected("DELAYED_TELEMETRY", deviceID) || fleet.affected("DELAYED_GATEWAY", deviceID)
		duplicated[observation.GetObservationId()] = fleet.affected("DUPLICATED_MESSAGES", deviceID)
	}
	fleet.lastSource = sourceTime
	batch, ok := fleet.publisher.(BatchPublisher)
	if !ok {
		return fleet.producers[0].producer.Flush(ctx, fleet.publisher)
	}
	return fleet.flushBatch(ctx, batch, deferred, duplicated)
}

func (fleet *Fleet) bufferPhysical(ctx context.Context, active map[string]gateway.Command, sourceTime time.Time, duration time.Duration) ([]*gridosv1.TelemetryObservation, error) {
	drafts := make([]gateway.ObservationDraft, 0, len(fleet.producers))
	observations := make([]*gridosv1.TelemetryObservation, 0, len(fleet.producers))
	for _, item := range fleet.producers {
		sample, err := fleet.physicalSample(item, active, sourceTime, duration)
		if err != nil {
			return nil, err
		}
		producer := item.producer
		offline := fleet.affected("OFFLINE_DEVICES", producer.deviceID) || fleet.affected("PARTIAL_REGION_OUTAGE", producer.deviceID)
		drafts = append(drafts, gateway.ObservationDraft{DeviceID: producer.deviceID, Build: func(sequence uint64) (gateway.BufferedObservation, error) {
			var observation *gridosv1.TelemetryObservation
			if offline {
				observation = producer.gapObservation(sequence, sourceTime)
			} else {
				observation, err = producer.sampleObservation(sequence, sample)
				if err != nil {
					return gateway.BufferedObservation{}, err
				}
			}
			payload, err := protojson.Marshal(observation)
			if err != nil {
				return gateway.BufferedObservation{}, err
			}
			observations = append(observations, observation)
			return gateway.BufferedObservation{ObservationID: observation.GetObservationId(), Payload: payload}, nil
		}})
	}
	if err := fleet.store.BufferObservationBatch(ctx, drafts); err != nil {
		return nil, err
	}
	return observations, nil
}

func (fleet *Fleet) activeCommands(ctx context.Context, sourceTime time.Time) (map[string]gateway.Command, error) {
	commands, err := fleet.store.ExecutableCommands(ctx, sourceTime)
	if err != nil {
		return nil, err
	}
	active := make(map[string]gateway.Command, len(commands))
	for _, command := range commands {
		current, found := active[command.DeviceID]
		if !found || command.Generation > current.Generation || command.Generation == current.Generation && command.EffectiveAt.After(current.EffectiveAt) {
			active[command.DeviceID] = command
		}
	}
	return active, nil
}

func (fleet *Fleet) physicalSample(item physicalProducer, active map[string]gateway.Command, sourceTime time.Time, duration time.Duration) (Sample, error) {
	load, err := fleet.profiles.LoadKW(item.device.LoadProfileType, sourceTime)
	if err != nil {
		return Sample{}, err
	}
	power := 0.0
	if command, found := active[item.device.DeviceID]; found {
		power = command.SetpointKW + load
	}
	parameters := item.device.Parameters
	available := math.Max(0, item.model.EnergyKWh()-parameters.HardwareFloorKWh-1e-9)
	room := math.Max(0, parameters.UsableEnergyKWh-item.model.EnergyKWh()-1e-9)
	discharge := math.Min(math.Max(power, 0), math.Min(parameters.MaxDischargeKW, available*parameters.DischargeEfficiency/duration.Hours()))
	charge := math.Min(math.Max(-power, 0), math.Min(parameters.MaxChargeKW, room/parameters.ChargeEfficiency/duration.Hours()))
	result, err := item.model.Step(battery.Input{ChargeKW: charge, DischargeKW: discharge, Duration: duration, CurrentLoadKW: load, GridConnectionVerified: true})
	if err != nil {
		return Sample{}, err
	}
	storageKW := discharge - charge
	return Sample{
		SourceTime: sourceTime, ObservationTime: sourceTime, FromGridKW: load - storageKW,
		FromStorageKW: storageKW, FromSolarKW: 0, NonSolarToHomeKW: load, ToHomeKW: load,
		StateOfEnergyPercent: result.EnergyKWh / parameters.UsableEnergyKWh * 100,
		State:                result.OperatingState, BackupHoursCurrent: result.BackupHoursCurrentUsage, BackupHours750W: result.BackupHours750W,
	}, nil
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
	confirmed := make([]string, 0, len(buffered))
	for _, item := range buffered {
		if deferred[item.ObservationID] {
			continue
		}
		confirmed = append(confirmed, item.ObservationID)
	}
	return fleet.store.ConfirmObservations(ctx, confirmed)
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
