package reconciliation

import (
	"reflect"
	"testing"
	"time"
)

func TestLateObservationClosesGapWithoutErasingEarlierKnowledge(t *testing.T) {
	window := Measurement{Begin: minutes(0), End: minutes(20), MaxGap: 10 * time.Minute}
	history := new(History)
	for _, observation := range []Telemetry{{PowerKW: 4, ObservedAt: minutes(0)}, {PowerKW: 4, ObservedAt: minutes(20)}} {
		if !history.Observe(observation) {
			t.Fatalf("observation at %v was not new", observation.ObservedAt)
		}
	}
	before := history.Delivery(window)
	nearKWh(t, before.DeliveredKWh, 0)
	if want := []Gap{{Begin: minutes(0), End: minutes(20)}}; !reflect.DeepEqual(before.Gaps, want) {
		t.Fatalf("gaps before the late observation = %v, want %v", before.Gaps, want)
	}

	if !history.Observe(Telemetry{PowerKW: 4, ObservedAt: minutes(10)}) {
		t.Fatal("late observation was not new")
	}
	after := history.Delivery(window)
	nearKWh(t, after.DeliveredKWh, 4.0/3)
	if len(after.Gaps) != 0 || after.Measured != 20*time.Minute {
		t.Fatalf("after the late observation: gaps = %v, measured = %v", after.Gaps, after.Measured)
	}
	want := []Telemetry{{PowerKW: 4, ObservedAt: minutes(0)}, {PowerKW: 4, ObservedAt: minutes(10)}, {PowerKW: 4, ObservedAt: minutes(20)}}
	if !reflect.DeepEqual(history.Observations(), want) {
		t.Fatalf("observations = %v, want %v", history.Observations(), want)
	}
}

func TestLateDuplicatesKeepTheFirstObservation(t *testing.T) {
	window := Measurement{Begin: minutes(0), End: minutes(20), MaxGap: 10 * time.Minute}
	history := new(History)
	for _, observation := range []Telemetry{{PowerKW: 4, ObservedAt: minutes(10)}, {PowerKW: 4, ObservedAt: minutes(0)}, {PowerKW: 4, ObservedAt: minutes(20)}} {
		history.Observe(observation)
	}
	if history.Observe(Telemetry{PowerKW: 4, ObservedAt: minutes(10)}) {
		t.Fatal("an identical duplicate counted as new knowledge")
	}
	if history.Observe(Telemetry{PowerKW: 9, ObservedAt: minutes(10)}) {
		t.Fatal("a conflicting duplicate replaced earlier knowledge")
	}
	got := history.Delivery(window)
	nearKWh(t, got.DeliveredKWh, 4.0/3)
	want := []Telemetry{{PowerKW: 4, ObservedAt: minutes(0)}, {PowerKW: 4, ObservedAt: minutes(10)}, {PowerKW: 4, ObservedAt: minutes(20)}}
	if !reflect.DeepEqual(history.Observations(), want) {
		t.Fatalf("observations = %v, want %v", history.Observations(), want)
	}
}

func TestLateArrivalOrderDoesNotChangeHistory(t *testing.T) {
	observations := []Telemetry{{PowerKW: 1, ObservedAt: minutes(0)}, {PowerKW: 2, ObservedAt: minutes(5)}, {PowerKW: 3, ObservedAt: minutes(10)}, {PowerKW: 4, ObservedAt: minutes(15)}}
	forward, reversed := new(History), new(History)
	for index := range observations {
		forward.Observe(observations[index])
		reversed.Observe(observations[len(observations)-1-index])
	}
	if !reflect.DeepEqual(forward.Observations(), reversed.Observations()) {
		t.Fatalf("forward = %v, reversed = %v", forward.Observations(), reversed.Observations())
	}
	if !reflect.DeepEqual(forward.Observations(), observations) {
		t.Fatalf("observations = %v, want %v", forward.Observations(), observations)
	}
}
