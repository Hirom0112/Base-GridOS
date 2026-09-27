package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/failures"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestScenarioFlagConfiguresFleetAndClock(t *testing.T) {
	directory := t.TempDir()
	fleetPath := filepath.Join(directory, "fleet.jsonl")
	if err := os.WriteFile(fleetPath, []byte("{\"device_id\":\"a\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(directory, "scenario.yaml")
	payload := fmt.Sprintf("name: flag\nclock:\n  seed: 41\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: %s\n  size: 1\ninjections: []\n", fleetPath)
	if err := os.WriteFile(scenarioPath, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := parseConfig([]string{"--scenario", scenarioPath, "--gateway-id", "gateway-1"})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.fleetPath != fleetPath || configuration.scenarioPath != scenarioPath {
		t.Fatalf("configuration=%+v", configuration)
	}
	want := time.Date(2026, time.August, 12, 16, 0, 0, 0, time.FixedZone("", -5*60*60))
	if !configuration.scenarioStart.Equal(want) {
		t.Fatalf("scenario start=%s want=%s", configuration.scenarioStart, want)
	}
}

func TestCadenceOverridesScenarioTickWithoutChangingLogicalClock(t *testing.T) {
	directory := t.TempDir()
	fleetPath := filepath.Join(directory, "fleet.jsonl")
	if err := os.WriteFile(fleetPath, []byte("{\"device_id\":\"a\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(directory, "scenario.yaml")
	payload := fmt.Sprintf("name: cadence\nclock:\n  seed: 41\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: %s\n  size: 1\ninjections: []\n", fleetPath)
	if err := os.WriteFile(scenarioPath, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := parseConfig([]string{"--scenario", scenarioPath, "--gateway-id", "gateway-1", "--cadence", "25ms"})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.telemetryCadence != 25*time.Millisecond {
		t.Fatalf("cadence=%s", configuration.telemetryCadence)
	}
	if configuration.scenarioTick != 5*time.Minute {
		t.Fatalf("scenario tick=%s", configuration.scenarioTick)
	}
}

func TestLiveScenarioUsesWallClockAndLiveCadence(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "scenarios", "heat-event-canonical.yaml")
	started := time.Now()
	configuration, err := parseConfig([]string{"--scenario", path, "--live", "--gateway-id", "gateway-live", "--cadence", "15s"})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.telemetryCadence != 15*time.Second || configuration.scenarioTick != 15*time.Second {
		t.Fatalf("live cadence=%s step=%s", configuration.telemetryCadence, configuration.scenarioTick)
	}
	if configuration.scenarioStart.Before(started) || configuration.scenarioStart.After(time.Now()) {
		t.Fatalf("scenario start=%s, want process wall clock", configuration.scenarioStart)
	}
}

func TestRuntimeDropsSelectedReceiptAfterDurableCommand(t *testing.T) {
	ctx := context.Background()
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	now := time.Now().UTC()
	scenario := failures.Scenario{Seed: 2, Start: now, Tick: time.Second, Injections: []failures.Injection{{At: now, Kind: failures.DroppedMessages}}}
	devices := []failures.Device{{ID: "a", Region: "LZ_AEN"}, {ID: "b", Region: "LZ_AEN"}}
	preview, err := failures.NewEngine(scenario, devices)
	if err != nil {
		t.Fatal(err)
	}
	selectedID := preview.Advance(now)[0].DeviceIDs[0]
	healthyID := "a"
	if selectedID == healthyID {
		healthyID = "b"
	}
	engine, err := failures.NewEngine(scenario, devices)
	if err != nil {
		t.Fatal(err)
	}
	runtime := failures.NewRuntime(engine)
	handler := newRuntimeCommandHandler(protocol.NewCommandHandler(store, "gateway", "token", func() time.Time { return now }), runtime, func() time.Time { return now })
	selected := connect.NewRequest(commandRequest(selectedID, now))
	selected.Header().Set("Authorization", "token")
	if _, err := handler.SubmitCommand(ctx, selected); connect.CodeOf(err) != connect.CodeDeadlineExceeded {
		t.Fatalf("selected response error=%v", err)
	}
	commands, err := store.Commands(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].DeviceID != selectedID {
		t.Fatalf("stored commands=%+v", commands)
	}
	healthy := connect.NewRequest(commandRequest(healthyID, now))
	healthy.Header().Set("Authorization", "token")
	if _, err := handler.SubmitCommand(ctx, healthy); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeDelayedGatewayTimesOutAfterDurableCommand(t *testing.T) {
	ctx := context.Background()
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	now := time.Now().UTC()
	scenario := failures.Scenario{Seed: 2, Start: now, Tick: time.Second, Injections: []failures.Injection{{At: now, Kind: failures.DelayedGateway}}}
	devices := []failures.Device{{ID: "a", Region: "LZ_AEN"}, {ID: "b", Region: "LZ_AEN"}}
	preview, err := failures.NewEngine(scenario, devices)
	if err != nil {
		t.Fatal(err)
	}
	selectedID := preview.Advance(now)[0].DeviceIDs[0]
	healthyID := "a"
	if selectedID == healthyID {
		healthyID = "b"
	}
	engine, err := failures.NewEngine(scenario, devices)
	if err != nil {
		t.Fatal(err)
	}
	runtime := failures.NewRuntime(engine)
	handler := newRuntimeCommandHandler(protocol.NewCommandHandler(store, "gateway", "token", func() time.Time { return now }), runtime, func() time.Time { return now })
	selected := connect.NewRequest(commandRequest(selectedID, now))
	selected.Header().Set("Authorization", "token")
	if _, err := handler.SubmitCommand(ctx, selected); connect.CodeOf(err) != connect.CodeDeadlineExceeded {
		t.Fatalf("delayed response error=%v", err)
	}
	commands, err := store.Commands(ctx)
	if err != nil || len(commands) != 1 || commands[0].DeviceID != selectedID {
		t.Fatalf("stored commands=%+v, error=%v", commands, err)
	}
	healthy := connect.NewRequest(commandRequest(healthyID, now))
	healthy.Header().Set("Authorization", "token")
	if _, err := handler.SubmitCommand(ctx, healthy); err != nil {
		t.Fatalf("healthy response error=%v", err)
	}
}

func TestRuntimeScheduledFaultFollowsAcceptedCommand(t *testing.T) {
	ctx := context.Background()
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	now := time.Now().UTC()
	scenario := failures.Scenario{Seed: 2, Start: now, Tick: time.Second, Injections: []failures.Injection{{At: now, Kind: failures.DroppedMessages, Scope: failures.Scheduled}}}
	engine, err := failures.NewEngine(scenario, []failures.Device{{ID: "a", Region: "LZ_AEN"}, {ID: "b", Region: "LZ_AEN"}})
	if err != nil {
		t.Fatal(err)
	}
	runtime := failures.NewRuntime(engine)
	handler := newRuntimeCommandHandler(protocol.NewCommandHandler(store, "gateway", "token", func() time.Time { return now }), runtime, func() time.Time { return now })
	selected := connect.NewRequest(commandRequest("a", now))
	selected.Msg.CommandIntent.EventId = "event-1"
	selected.Header().Set("Authorization", "token")
	if _, err := handler.SubmitCommand(ctx, selected); connect.CodeOf(err) != connect.CodeDeadlineExceeded {
		t.Fatalf("selected response error=%v", err)
	}
	commands, err := store.Commands(ctx)
	if err != nil || len(commands) != 1 || commands[0].DeviceID != "a" {
		t.Fatalf("stored commands=%+v, error=%v", commands, err)
	}
	healthy := connect.NewRequest(commandRequest("b", now))
	healthy.Msg.CommandIntent.EventId = "other-event"
	healthy.Header().Set("Authorization", "token")
	if _, err := handler.SubmitCommand(ctx, healthy); err != nil {
		t.Fatalf("other event was affected: %v", err)
	}
}

func commandRequest(deviceID string, now time.Time) *gridosv1.SubmitCommandRequest {
	return &gridosv1.SubmitCommandRequest{CommandIntent: &gridosv1.CommandIntent{
		CommandId: "command-" + deviceID, IdempotencyKey: "key-" + deviceID, DeviceId: deviceID, Generation: 1,
		SetpointKw: 1, EffectiveAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(time.Minute)),
	}}
}
