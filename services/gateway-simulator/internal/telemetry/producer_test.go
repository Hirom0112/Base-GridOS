package telemetry

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
)

func TestObservationsCarryTruthfulSemantics(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	receiveTime := time.Date(2026, 8, 12, 18, 0, 2, 0, time.UTC)
	producer, err := NewProducer(store, "device-1", gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, func() time.Time { return receiveTime })
	if err != nil {
		t.Fatal(err)
	}
	sourceTime := receiveTime.Add(-2 * time.Second)
	observationTime := receiveTime.Add(-time.Second)
	first, err := producer.Observe(ctx, Sample{
		SourceTime:           sourceTime,
		ObservationTime:      observationTime,
		FromGridKW:           1.1,
		FromStorageKW:        2.2,
		FromSolarKW:          0.7,
		NonSolarToHomeKW:     3.3,
		ToHomeKW:             4,
		StateOfEnergyPercent: 74,
		State:                battery.OnGrid,
		BackupHoursCurrent:   4.8052,
		BackupHours750W:      19.8613,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.GetSourceTime().AsTime() != sourceTime || first.GetReceiveTime().AsTime() != receiveTime || first.GetObservationTime().AsTime() != observationTime {
		t.Fatalf("times source=%v receive=%v observation=%v", first.GetSourceTime(), first.GetReceiveTime(), first.GetObservationTime())
	}
	if first.GetSequence() != 1 || first.GetUnits() != gridosv1.TelemetryUnits_TELEMETRY_UNITS_KW_PERCENT_VOLTS || first.GetMeasurementBoundary() != gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT {
		t.Fatalf("observation metadata=%+v", first)
	}
	if first.GetPowerFlow().GetToHomeKw() != first.GetPowerFlow().GetFromGridKw()+first.GetPowerFlow().GetFromStorageKw()+first.GetPowerFlow().GetFromSolarKw() {
		t.Fatalf("unbalanced power flow=%+v", first.GetPowerFlow())
	}
	if first.GetStateOfEnergyPercent() != 74 || first.GetOnGrid() == nil || first.GetValueState() != gridosv1.ValueState_VALUE_STATE_PRESENT {
		t.Fatalf("state=%+v", first)
	}
	second, err := producer.Observe(ctx, Sample{SourceTime: sourceTime.Add(time.Second), ObservationTime: observationTime.Add(time.Second), State: battery.OnGrid})
	if err != nil {
		t.Fatal(err)
	}
	if second.GetSequence() != 2 {
		t.Fatalf("sequence=%d", second.GetSequence())
	}
}

func TestGapIsMissingAndUnavailable(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	producer, err := NewProducer(store, "device-1", gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	gap, err := producer.Gap(ctx, now.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if gap.GetValueState() != gridosv1.ValueState_VALUE_STATE_MISSING || gap.GetTelemetryUnavailable() == nil || gap.GetPowerFlow() != nil {
		t.Fatalf("gap=%+v", gap)
	}
}

func TestBufferRetainedUntilConfirmedReceipt(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	producer, err := NewProducer(store, "device-1", gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := producer.Observe(ctx, Sample{SourceTime: now.Add(-time.Second), ObservationTime: now, State: battery.OnGrid}); err != nil {
		t.Fatal(err)
	}
	publisher := &recordingPublisher{err: errors.New("offline")}
	if err := producer.Flush(ctx, publisher); !errors.Is(err, publisher.err) {
		t.Fatalf("flush error=%v", err)
	}
	buffered, err := store.BufferedObservations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 1 {
		t.Fatalf("buffered after failure=%d", len(buffered))
	}
	publisher.err = nil
	if err := producer.Flush(ctx, publisher); err != nil {
		t.Fatal(err)
	}
	buffered, err = store.BufferedObservations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 0 || len(publisher.observations) != 2 {
		t.Fatalf("buffered=%d attempts=%d", len(buffered), len(publisher.observations))
	}
}

type recordingPublisher struct {
	observations []*gridosv1.TelemetryObservation
	err          error
}

func (publisher *recordingPublisher) Publish(_ context.Context, observation *gridosv1.TelemetryObservation) error {
	publisher.observations = append(publisher.observations, observation)
	return publisher.err
}

func openStore(t *testing.T, ctx context.Context) *gateway.Store {
	t.Helper()
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}
