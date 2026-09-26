package tests

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

func TestRestartRetainsCommandsAndDeliversTelemetryOnce(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "gateway.db")
	process := exec.Command(os.Args[0], "-test.run=TestGatewayProcessHelper")
	process.Env = append(os.Environ(), "GRIDOS_RESTART_HELPER=1", "GRIDOS_RESTART_DATABASE="+databasePath)
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "ready\n" {
		t.Fatalf("helper output=%q", line)
	}
	if err := process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := process.Wait(); err == nil {
		t.Fatal("killed process exited successfully")
	}
	ctx := context.Background()
	store, err := gateway.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	now := restartTime().Add(time.Minute)
	commands, err := store.ClaimExecutableCommands(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].CommandID != "command-restart" {
		t.Fatalf("retained commands=%+v", commands)
	}
	commands, err = store.ClaimExecutableCommands(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 0 {
		t.Fatalf("command executed twice: %+v", commands)
	}
	producer, err := telemetry.NewProducer(store, "device-1", gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, restartTime)
	if err != nil {
		t.Fatal(err)
	}
	publisher := &restartPublisher{}
	if err := producer.Flush(ctx, publisher); err != nil {
		t.Fatal(err)
	}
	if err := producer.Flush(ctx, publisher); err != nil {
		t.Fatal(err)
	}
	if len(publisher.observations) != 1 {
		t.Fatalf("telemetry deliveries=%d", len(publisher.observations))
	}
}

func TestGatewayProcessHelper(t *testing.T) {
	if os.Getenv("GRIDOS_RESTART_HELPER") != "1" {
		return
	}
	ctx := context.Background()
	store, err := gateway.Open(ctx, os.Getenv("GRIDOS_RESTART_DATABASE"))
	if err != nil {
		t.Fatal(err)
	}
	now := restartTime()
	command := gateway.Command{CommandID: "command-restart", IdempotencyKey: "restart", DeviceID: "device-1", Generation: 1, SetpointKW: 2, EffectiveAt: now, ExpiresAt: now.Add(time.Hour)}
	if acknowledgement, err := store.AcceptCommand(ctx, command, now); err != nil || !acknowledgement.Accepted {
		t.Fatalf("acknowledgement=%+v error=%v", acknowledgement, err)
	}
	producer, err := telemetry.NewProducer(store, "device-1", gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, restartTime)
	if err != nil {
		t.Fatal(err)
	}
	_, err = producer.Observe(ctx, telemetry.Sample{SourceTime: now.Add(-time.Second), ObservationTime: now, State: battery.OnGrid})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ready")
	time.Sleep(time.Minute)
}

func restartTime() time.Time {
	return time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
}

type restartPublisher struct {
	observations []*gridosv1.TelemetryObservation
}

func (publisher *restartPublisher) Publish(_ context.Context, observation *gridosv1.TelemetryObservation) error {
	publisher.observations = append(publisher.observations, observation)
	return nil
}
