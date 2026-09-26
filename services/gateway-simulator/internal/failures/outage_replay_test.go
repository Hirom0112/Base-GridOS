package failures

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

func TestOutageReplayPreservesSequenceAndSourceTimeOnce(t *testing.T) {
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
	start := time.Date(2026, time.August, 12, 18, 0, 0, 0, time.UTC)
	producer, err := telemetry.NewProducer(store, "device-1", gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, func() time.Time { return start.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	recorder := &observationRecorder{}
	network, err := NewNetwork(recorder)
	if err != nil {
		t.Fatal(err)
	}
	network.Disconnect()
	for offset := range 3 {
		sourceTime := start.Add(time.Duration(offset) * time.Minute)
		_, err := producer.Observe(ctx, telemetry.Sample{
			SourceTime: sourceTime, ObservationTime: sourceTime,
			FromGridKW: 1, ToHomeKW: 1, StateOfEnergyPercent: 50,
			GridVoltageVolts: 240, State: "ON_GRID",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := producer.Flush(ctx, network); err == nil {
		t.Fatal("flush during outage succeeded")
	}
	if err := network.Restore(ctx, producer); err != nil {
		t.Fatal(err)
	}
	if err := network.Restore(ctx, producer); err != nil {
		t.Fatal(err)
	}
	if len(recorder.observations) != 3 {
		t.Fatalf("observations=%d", len(recorder.observations))
	}
	for index, observation := range recorder.observations {
		if observation.GetSequence() != uint64(index+1) {
			t.Fatalf("sequence=%d index=%d", observation.GetSequence(), index)
		}
		want := start.Add(time.Duration(index) * time.Minute)
		if !observation.GetSourceTime().AsTime().Equal(want) {
			t.Fatalf("source time=%s want=%s", observation.GetSourceTime().AsTime(), want)
		}
	}
}

type observationRecorder struct {
	observations []*gridosv1.TelemetryObservation
}

func (recorder *observationRecorder) Publish(_ context.Context, observation *gridosv1.TelemetryObservation) error {
	recorder.observations = append(recorder.observations, observation)
	return nil
}
