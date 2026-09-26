package reconciliation

import (
	"math"
	"reflect"
	"testing"
	"time"
)

var eventStart = time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC)

func minutes(offset int) time.Time {
	return eventStart.Add(time.Duration(offset) * time.Minute)
}

func hourWindow() Measurement {
	return Measurement{Begin: eventStart, End: minutes(60), MaxGap: 10 * time.Minute}
}

func nearKWh(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("delivered = %v kWh, want %v kWh", got, want)
	}
}

func TestGapsStayUnknownWhileMeasuredZeroCounts(t *testing.T) {
	observations := []Telemetry{
		{PowerKW: 4, ObservedAt: minutes(0)},
		{PowerKW: 4, ObservedAt: minutes(10)},
		{PowerKW: 6, ObservedAt: minutes(20)},
		{PowerKW: 2, ObservedAt: minutes(45)},
		{PowerKW: 2, ObservedAt: minutes(50)},
		{PowerKW: 0, ObservedAt: minutes(55)},
		{PowerKW: 0, ObservedAt: minutes(60)},
		{PowerKW: 0, ObservedAt: minutes(62)},
	}
	got := Integrate(hourWindow(), observations)
	nearKWh(t, got.DeliveredKWh, 4.0/6+4.0/6+2.0/12+2.0/12)
	if got.Measured != 35*time.Minute {
		t.Fatalf("measured = %v, want 35m", got.Measured)
	}
	wantGaps := []Gap{{Begin: minutes(20), End: minutes(45)}}
	if !reflect.DeepEqual(got.Gaps, wantGaps) {
		t.Fatalf("gaps = %v, want %v", got.Gaps, wantGaps)
	}
}

func TestGapsCoverUnobservedEdgesOfTheWindow(t *testing.T) {
	cases := []struct {
		name         string
		observations []Telemetry
		deliveredKWh float64
		measured     time.Duration
		gaps         []Gap
	}{
		{
			name:     "no observations leave the whole window unknown",
			measured: 0,
			gaps:     []Gap{{Begin: minutes(0), End: minutes(60)}},
		},
		{
			name: "an unclosed final observation is not held to the window end",
			observations: []Telemetry{
				{PowerKW: 4, ObservedAt: minutes(0)},
				{PowerKW: 4, ObservedAt: minutes(5)},
			},
			deliveredKWh: 4.0 / 12,
			measured:     5 * time.Minute,
			gaps:         []Gap{{Begin: minutes(5), End: minutes(60)}},
		},
		{
			name: "an observation before the window holds into it",
			observations: []Telemetry{
				{PowerKW: 4, ObservedAt: minutes(-2)},
				{PowerKW: 3, ObservedAt: minutes(3)},
				{PowerKW: 3, ObservedAt: minutes(9)},
			},
			deliveredKWh: 4.0/20 + 3.0/10,
			measured:     9 * time.Minute,
			gaps:         []Gap{{Begin: minutes(9), End: minutes(60)}},
		},
		{
			name: "consecutive gaps merge",
			observations: []Telemetry{
				{PowerKW: 4, ObservedAt: minutes(12)},
				{PowerKW: 4, ObservedAt: minutes(30)},
				{PowerKW: 4, ObservedAt: minutes(35)},
			},
			deliveredKWh: 4.0 / 12,
			measured:     5 * time.Minute,
			gaps:         []Gap{{Begin: minutes(0), End: minutes(30)}, {Begin: minutes(35), End: minutes(60)}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Integrate(hourWindow(), testCase.observations)
			nearKWh(t, got.DeliveredKWh, testCase.deliveredKWh)
			if got.Measured != testCase.measured {
				t.Fatalf("measured = %v, want %v", got.Measured, testCase.measured)
			}
			if !reflect.DeepEqual(got.Gaps, testCase.gaps) {
				t.Fatalf("gaps = %v, want %v", got.Gaps, testCase.gaps)
			}
		})
	}
}
