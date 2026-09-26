package report

import (
	"context"
	"reflect"
	"testing"
)

type storedReportSource struct {
	eventID string
	data    StoredEvent
}

func (source *storedReportSource) EventReportData(_ context.Context, eventID string) (StoredEvent, error) {
	source.eventID = eventID
	return source.data, nil
}

func TestBuildEventReportFromStorage(t *testing.T) {
	source := &storedReportSource{data: StoredEvent{
		RequestedMW:    20,
		ApprovedMW:     18,
		CommandedMW:    17.5,
		AcknowledgedMW: 16,
		Exclusions: map[string]uint64{
			"UNDER_RESERVE": 12,
			"STALE":         3,
		},
		Provenance: []string{"simulated", "derived"},
		Versions:   Versions{Policy: "policy-7", Solver: "fallback-1", Model: "load-3"},
	}}

	got, err := Build(context.Background(), source, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if source.eventID != "event-1" {
		t.Fatalf("storage event ID = %q, want event-1", source.eventID)
	}
	want := EventReport{
		EventID:          "event-1",
		RequestedMW:      20,
		ApprovedMW:       18,
		CommandedMW:      17.5,
		AcknowledgedMW:   16,
		ExcludedByReason: map[string]uint64{"UNDER_RESERVE": 12, "STALE": 3},
		Provenance:       []string{"simulated", "derived"},
		Versions:         Versions{Policy: "policy-7", Solver: "fallback-1", Model: "load-3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report = %#v, want %#v", got, want)
	}
}
