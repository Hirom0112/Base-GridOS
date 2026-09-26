package report

import (
	"context"
	"reflect"
	"testing"
	"time"
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

func TestBuildEventReportIncludesDelivered(t *testing.T) {
	begin := time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC)
	delivered := &Delivered{
		DeliveredMWh:    1.25,
		DeliveredMW:     2.5,
		TrackingErrorMW: -0.1,
		ResponseLatency: 45 * time.Second,
		Completeness:    0.9,
		Responded:       3,
		Commanded:       4,
		UncertainIntervals: []UncertainInterval{
			{DeviceID: "device-1", Begin: begin, End: begin.Add(30 * time.Minute), Bounds: &PowerBounds{LowerKW: 2, UpperKW: 5}},
			{DeviceID: "device-2", Begin: begin.Add(20 * time.Minute), End: begin.Add(45 * time.Minute)},
		},
	}
	source := &storedReportSource{data: StoredEvent{RequestedMW: 20, Delivered: delivered}}

	got, err := Build(context.Background(), source, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Delivered, delivered) {
		t.Fatalf("delivered = %#v, want %#v", got.Delivered, delivered)
	}
	if got.Delivered == delivered || &got.Delivered.UncertainIntervals[0] == &delivered.UncertainIntervals[0] {
		t.Fatal("report aliases the stored delivery")
	}
}

func TestBuildEventReportWithoutDeliveredStaysUnverified(t *testing.T) {
	source := &storedReportSource{data: StoredEvent{RequestedMW: 20}}
	got, err := Build(context.Background(), source, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Delivered != nil {
		t.Fatalf("delivered = %#v, want nil before verification", got.Delivered)
	}
}
