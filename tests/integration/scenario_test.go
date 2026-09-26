package integration

import (
	"testing"
	"time"
)

func TestScenarioFitsLiveMeasurementWindow(t *testing.T) {
	start := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	scenario := loadScenario(t, repositoryRoot(t), "lost-ack-still-executing")
	runtime := scenario.retime(start)
	if got := runtime.Event.StartAt; !got.Equal(start.Add(10 * time.Second)) {
		t.Fatalf("event begin = %s", got)
	}
	if got := runtime.Event.EndAt; !got.Equal(start.Add(70 * time.Second)) {
		t.Fatalf("event end = %s", got)
	}
	if got := runtime.Injections[0].At; !got.Equal(start.Add(12*time.Second + 500*time.Millisecond)) {
		t.Fatalf("first injection = %s", got)
	}
	if runtime.Clock.StartAt != start || runtime.Clock.IntervalSeconds != 5 {
		t.Fatalf("runtime clock = %#v", runtime.Clock)
	}
}
