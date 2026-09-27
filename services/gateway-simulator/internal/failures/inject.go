package failures

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Device struct {
	ID          string
	Region      string
	ScheduledKW float64
}

type Effect struct {
	At        time.Time
	Kind      Kind
	Scope     Scope
	DeviceIDs []string
	Region    string
	LostMW    float64
}

type Engine struct {
	scenario  Scenario
	devices   []Device
	commanded map[string]map[string]struct{}
	next      int
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
		if err := validateInjection(injection, scenario, devices); err != nil {
			return nil, err
		}
	}
	sortedDevices := append([]Device(nil), devices...)
	sort.Slice(sortedDevices, func(i, j int) bool { return sortedDevices[i].ID < sortedDevices[j].ID })
	sort.SliceStable(scenario.Injections, func(i, j int) bool { return scenario.Injections[i].At.Before(scenario.Injections[j].At) })
	return &Engine{scenario: scenario, devices: sortedDevices, commanded: make(map[string]map[string]struct{})}, nil
}

func validateInjection(injection Injection, scenario Scenario, devices []Device) error {
	if !validKind(injection.Kind) {
		return fmt.Errorf("unsupported injection %q", injection.Kind)
	}
	if injection.Scope != "" && injection.Scope != Scheduled && injection.Scope != NextCommand {
		return fmt.Errorf("unsupported injection scope %q", injection.Scope)
	}
	if injection.Scope != "" && (globalKind(injection.Kind) || injection.Kind == PartialRegionOutage) {
		return fmt.Errorf("%s scope cannot target %q", injection.Scope, injection.Kind)
	}
	if injection.Region != "" && injection.Kind != PartialRegionOutage {
		return fmt.Errorf("region cannot target %q", injection.Kind)
	}
	if injection.Region != "" {
		found := false
		for _, device := range devices {
			if device.Region == injection.Region {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("region %q has no devices", injection.Region)
		}
	}
	elapsed := injection.At.Sub(scenario.Start)
	if elapsed < 0 || elapsed%scenario.Tick != 0 {
		return fmt.Errorf("injection %q is outside the scenario clock", injection.Kind)
	}
	return nil
}

func (engine *Engine) Advance(now time.Time) []Effect {
	return engine.advance(now, "")
}

func (engine *Engine) recordCommand(eventID, deviceID string, setpointKW float64) {
	devices := engine.commanded[eventID]
	if setpointKW == 0 {
		delete(devices, deviceID)
		if len(devices) == 0 {
			delete(engine.commanded, eventID)
		}
		return
	}
	if devices == nil {
		devices = make(map[string]struct{})
		engine.commanded[eventID] = devices
	}
	devices[deviceID] = struct{}{}
}

func (engine *Engine) advance(now time.Time, eventID string) []Effect {
	var effects []Effect
	commanded := engine.commanded[eventID]
	if eventID == "" {
		commanded = make(map[string]struct{})
		for _, devices := range engine.commanded {
			for deviceID := range devices {
				commanded[deviceID] = struct{}{}
			}
		}
	}
	for engine.next < len(engine.scenario.Injections) {
		injection := engine.scenario.Injections[engine.next]
		if injection.At.After(now) {
			break
		}
		if injection.Scope == Scheduled && len(commanded) == 0 {
			break
		}
		effects = append(effects, engine.effect(injection, commanded))
		engine.next++
	}
	return effects
}

func (engine *Engine) effect(injection Injection, commanded map[string]struct{}) Effect {
	effect := Effect{At: injection.At, Kind: injection.Kind, Scope: injection.Scope}
	if globalKind(injection.Kind) {
		return effect
	}
	if injection.Scope == NextCommand {
		return effect
	}
	if injection.Scope == Scheduled {
		ids := make([]string, 0, len(commanded))
		for deviceID := range commanded {
			ids = append(ids, deviceID)
		}
		sort.Strings(ids)
		digest := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", engine.scenario.Seed, injection.Kind, strings.Join(ids, "\x00"))))
		index := binary.BigEndian.Uint64(digest[:8]) % uint64(len(ids))
		effect.DeviceIDs = []string{ids[index]}
		return effect
	}
	if injection.Kind != PartialRegionOutage {
		device := engine.devices[engine.seededIndex(injection, len(engine.devices))]
		effect.DeviceIDs = []string{device.ID}
		return effect
	}
	effect.Region = injection.Region
	if effect.Region == "" {
		device := engine.devices[engine.seededIndex(injection, len(engine.devices))]
		effect.Region = device.Region
	}
	var regional []Device
	for _, candidate := range engine.devices {
		if candidate.Region == effect.Region {
			regional = append(regional, candidate)
		}
	}
	start := engine.seededIndex(injection, len(regional))
	count := (len(regional) + 4) / 5
	for offset := range count {
		affected := regional[(start+offset)%len(regional)]
		effect.DeviceIDs = append(effect.DeviceIDs, affected.ID)
		effect.LostMW += affected.ScheduledKW / 1000
	}
	return effect
}

func (engine *Engine) seededIndex(injection Injection, length int) int {
	payload := fmt.Sprintf("%d|%s|%s", engine.scenario.Seed, injection.At.Format(time.RFC3339Nano), injection.Kind)
	digest := sha256.Sum256([]byte(payload))
	return int(binary.BigEndian.Uint64(digest[:8]) % uint64(length))
}

func globalKind(kind Kind) bool {
	return kind == GatewayRestart || kind == BadForecasts || kind == OptimizerTimeout
}

func validKind(kind Kind) bool {
	switch kind {
	case OfflineDevices, DelayedGateway, DelayedTelemetry, DroppedMessages, DuplicatedMessages, GatewayRestart, PartialRegionOutage, BadForecasts, HotBatteries, StaleState, OptimizerTimeout:
		return true
	default:
		return false
	}
}
