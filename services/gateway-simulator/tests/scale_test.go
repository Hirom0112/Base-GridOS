package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

type scaleDevice struct {
	DeviceID string `json:"device_id"`
}

type scaleRecorder struct {
	sequences map[string]uint64
	count     int
}

func (recorder *scaleRecorder) Publish(_ context.Context, observation *gridosv1.TelemetryObservation) error {
	want := recorder.sequences[observation.GetDeviceId()] + 1
	if observation.GetSequence() != want {
		return fmt.Errorf("device %s sequence %d, want %d", observation.GetDeviceId(), observation.GetSequence(), want)
	}
	recorder.sequences[observation.GetDeviceId()] = want
	recorder.count++
	return nil
}

func TestScale5000(t *testing.T) {
	ctx := context.Background()
	deviceIDs := scaleDeviceIDs(t)
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	recorder := &scaleRecorder{sequences: make(map[string]uint64, len(deviceIDs))}
	fleet, err := telemetry.NewFleet(store, deviceIDs, 5*time.Second, recorder)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	for interval := range 120 {
		if err := fleet.Emit(ctx, start.Add(time.Duration(interval)*5*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if recorder.count != 600000 || len(recorder.sequences) != 5000 {
		t.Fatalf("observations=%d devices=%d", recorder.count, len(recorder.sequences))
	}
	for deviceID, sequence := range recorder.sequences {
		if sequence != 120 {
			t.Fatalf("device %s final sequence=%d", deviceID, sequence)
		}
	}
}

func scaleDeviceIDs(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	path := filepath.Join(filepath.Dir(file), "../../../testdata/fleets/austin-5000.jsonl")
	handle, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	deviceIDs := make([]string, 0, 5000)
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		var device scaleDevice
		if err := json.Unmarshal(scanner.Bytes(), &device); err != nil {
			t.Fatal(err)
		}
		deviceIDs = append(deviceIDs, device.DeviceID)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(deviceIDs) != 5000 {
		t.Fatalf("devices=%d", len(deviceIDs))
	}
	return deviceIDs
}
