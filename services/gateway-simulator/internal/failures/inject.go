package failures

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Device struct {
	ID          string
	Region      string
	ScheduledKW float64
}

type Effect struct {
	Kind      Kind
	DeviceIDs []string
	Region    string
}

type Engine struct {
	scenario Scenario
	devices  []Device
	next     int
}

func NewEngine(scenario Scenario, devices []Device) (*Engine, error) {
	if scenario.Start.IsZero() || scenario.Tick <= 0 || len(devices) == 0 {
		return nil, errors.New("scenario clock and devices are required")
	}
	seen := make(map[string]struct{}, len(devices))
	for _, device := range devices {
		if device.ID == "" || device.Region == "" {
			return nil, errors.New("device identifier and region are required")
		}
		if _, exists := seen[device.ID]; exists {
			return nil, fmt.Errorf("duplicate device %q", device.ID)
		}
		seen[device.ID] = struct{}{}
	}
	for _, injection := range scenario.Injections {
		if !validKind(injection.Kind) {
			return nil, fmt.Errorf("unsupported injection %q", injection.Kind)
		}
		elapsed := injection.At.Sub(scenario.Start)
		if elapsed < 0 || elapsed%scenario.Tick != 0 {
			return nil, fmt.Errorf("injection %q is outside the scenario clock", injection.Kind)
		}
	}
	sortedDevices := append([]Device(nil), devices...)
	sort.Slice(sortedDevices, func(i, j int) bool { return sortedDevices[i].ID < sortedDevices[j].ID })
	sort.SliceStable(scenario.Injections, func(i, j int) bool { return scenario.Injections[i].At.Before(scenario.Injections[j].At) })
	return &Engine{scenario: scenario, devices: sortedDevices}, nil
}

func (engine *Engine) Advance(now time.Time) []Effect {
	var effects []Effect
	for engine.next < len(engine.scenario.Injections) {
		injection := engine.scenario.Injections[engine.next]
		if injection.At.After(now) {
			break
		}
		effects = append(effects, engine.effect(injection))
		engine.next++
	}
	return effects
}

func (engine *Engine) effect(injection Injection) Effect {
	effect := Effect{Kind: injection.Kind}
	if globalKind(injection.Kind) {
		return effect
	}
	device := engine.devices[engine.seededIndex(injection, len(engine.devices))]
	if injection.Kind != PartialRegionOutage {
		effect.DeviceIDs = []string{device.ID}
		return effect
	}
	effect.Region = device.Region
	for _, candidate := range engine.devices {
		if candidate.Region == effect.Region {
			effect.DeviceIDs = append(effect.DeviceIDs, candidate.ID)
		}
	}
	return effect
}

func (engine *Engine) seededIndex(injection Injection, length int) int {
	payload := fmt.Sprintf("%d|%s|%s", engine.scenario.Seed, injection.At.Format(time.RFC3339Nano), injection.Kind)
	digest := sha256.Sum256([]byte(payload))
	return int(binary.BigEndian.Uint64(digest[:8]) % uint64(length))
}

func globalKind(kind Kind) bool {
	return kind == DelayedGateway || kind == GatewayRestart || kind == BadForecasts || kind == OptimizerTimeout
}

func validKind(kind Kind) bool {
	switch kind {
	case OfflineDevices, DelayedGateway, DelayedTelemetry, DroppedMessages, DuplicatedMessages, GatewayRestart, PartialRegionOutage, BadForecasts, HotBatteries, StaleState, OptimizerTimeout:
		return true
	default:
		return false
	}
}
