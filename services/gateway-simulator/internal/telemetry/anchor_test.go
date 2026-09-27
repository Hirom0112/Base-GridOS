package telemetry

import (
	"testing"
	"time"
)

func TestAnchorSkipsMissedCadenceSlots(t *testing.T) {
	wallStart := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	logicalStart := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	cadence := 15 * time.Second
	sourceStep := 5 * time.Minute
	first := sourceAt(logicalStart, wallStart, wallStart, cadence, sourceStep)
	if !first.Equal(logicalStart) {
		t.Fatalf("first source time = %s", first)
	}
	late := sourceAt(logicalStart, wallStart, wallStart.Add(46*time.Second), cadence, sourceStep)
	if !late.Equal(logicalStart.Add(3 * sourceStep)) {
		t.Fatalf("late source time = %s", late)
	}
	if !late.After(first.Add(sourceStep)) {
		t.Fatalf("missed cadence slots were replayed: %s", late)
	}
}
