package load

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ingestResult struct {
	persisted    int64
	dropped      int64
	sequenceGaps int64
}

func runTelemetryIngest(t *testing.T, deviceCount int, cadence, duration time.Duration) ingestResult {
	t.Helper()
	stack := startLoadStack(t)
	devices := loadDeviceIDs(t, stack.root)
	if len(devices) != deviceCount {
		t.Fatalf("fleet has %d devices, want %d", len(devices), deviceCount)
	}
	client := gridosv1connect.NewTelemetryServiceClient(&http.Client{Timeout: cadence}, stack.controlURL)
	ticks := int(duration / cadence)
	started := time.Now().UTC()
	for tick := 1; tick <= ticks; tick++ {
		at := started.Add(time.Duration(tick-1) * cadence)
		if wait := time.Until(at); wait > 0 {
			time.Sleep(wait)
		}
		ctx, cancel := context.WithDeadline(context.Background(), at.Add(cadence))
		err := publishLoadTick(ctx, client, devices, uint64(tick), at, 74)
		cancel()
		if err != nil {
			t.Fatalf("tick %d at %s: %v", tick, at.Format(time.RFC3339Nano), err)
		}
	}
	if wait := time.Until(started.Add(duration)); wait > 0 {
		time.Sleep(wait)
	}
	var result ingestResult
	var distinctDevices int64
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := stack.database.QueryRow(ctx, `SELECT COALESCE(sum(observations), 0), count(*),
		COALESCE(sum(last_sequence - first_sequence + 1 - observations), 0)
		FROM (SELECT device_id, count(*) AS observations, min(sequence) AS first_sequence,
		max(sequence) AS last_sequence FROM telemetry_observations GROUP BY device_id) AS per_device`).Scan(
		&result.persisted, &distinctDevices, &result.sequenceGaps)
	if err != nil {
		t.Fatal(err)
	}
	result.dropped = int64(deviceCount*ticks) - result.persisted
	if distinctDevices != int64(deviceCount) {
		t.Fatalf("persisted devices=%d, want %d", distinctDevices, deviceCount)
	}
	return result
}

func publishLoadTick(ctx context.Context, client gridosv1connect.TelemetryServiceClient, devices []string, sequence uint64, at time.Time, energyPercent float64) error {
	const batchSize = 250
	const workers = 4
	jobs := make(chan int)
	errors := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for start := range jobs {
				end := min(start+batchSize, len(devices))
				if err := publishLoadBatch(ctx, client, devices[start:end], sequence, at, energyPercent); err != nil {
					errors <- err
					return
				}
			}
		}()
	}
	for start := 0; start < len(devices); start += batchSize {
		jobs <- start
	}
	close(jobs)
	group.Wait()
	close(errors)
	for err := range errors {
		return err
	}
	return nil
}

func publishLoadBatch(ctx context.Context, client gridosv1connect.TelemetryServiceClient, devices []string, sequence uint64, at time.Time, energyPercent float64) error {
	observations := make([]*gridosv1.TelemetryObservation, 0, len(devices))
	for _, device := range devices {
		observations = append(observations, &gridosv1.TelemetryObservation{
			ObservationId: fmt.Sprintf("load-%s-%d", device, sequence), DeviceId: device,
			Sequence: sequence, ObservationTime: timestamppb.New(at), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT,
			StateOfEnergyPercent: energyPercent,
			OperatingState:       &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(at), EstimatedBackupHoursAtCurrentUsage: 4}},
		})
	}
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: "load-gateway", Observations: observations})
	request.Header().Set("Authorization", loadGatewayToken)
	response, err := client.PublishTelemetry(ctx, request)
	if err != nil {
		return err
	}
	if response.Msg.GetDurableReceiptId() == "" || len(response.Msg.GetObservationIds()) != len(observations) {
		return fmt.Errorf("durable receipt acknowledged %d of %d observations", len(response.Msg.GetObservationIds()), len(observations))
	}
	return nil
}

func loadDeviceIDs(t *testing.T, root string) []string {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "testdata/fleets/austin-5000.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ids := make([]string, 0, 5000)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var device struct {
			ID string `json:"device_id"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &device); err != nil {
			t.Fatal(err)
		}
		if device.ID == "" {
			t.Fatal("fleet device identifier is empty")
		}
		ids = append(ids, device.ID)
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}
