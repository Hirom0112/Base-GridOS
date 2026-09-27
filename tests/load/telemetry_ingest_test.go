package load

import (
	"testing"
	"time"
)

func TestTelemetryIngest(t *testing.T) {
	if testing.Short() {
		t.Skip("ten-minute ingest load run")
	}
	result := runTelemetryIngest(t, 5000, 5*time.Second, 10*time.Minute)
	if result.persisted != 600000 || result.dropped != 0 || result.sequenceGaps != 0 {
		t.Fatalf("persisted=%d dropped=%d sequence_gaps=%d", result.persisted, result.dropped, result.sequenceGaps)
	}
	t.Logf("devices=5000 cadence=5s duration=10m persisted=%d dropped=%d sequence_gaps=%d", result.persisted, result.dropped, result.sequenceGaps)
}
