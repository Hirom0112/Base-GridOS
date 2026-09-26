package reconciliation

import (
	"testing"
	"time"
)

func TestNoOvershootUntilExpiryOrProof(t *testing.T) {
	send := lostAcknowledgement()
	interval, err := FeasibleInterval(send)
	if err != nil {
		t.Fatal(err)
	}
	effective := send.PossiblyAccepted.EffectiveAt
	cases := []struct {
		name         string
		now          time.Time
		observations []Telemetry
		standing     Standing
		countedKW    float64
		replaceKW    float64
	}{
		{
			name:     "no telemetry inside the window keeps the upper bound reserved",
			now:      effective.Add(time.Minute),
			standing: PossiblyOperating, countedKW: 5, replaceKW: 0,
		},
		{
			name:         "telemetry at the last confirmed setpoint proves nothing",
			now:          effective.Add(2 * time.Minute),
			observations: []Telemetry{{PowerKW: 1.9, ObservedAt: effective.Add(90 * time.Second)}},
			standing:     PossiblyOperating, countedKW: 5, replaceKW: 0,
		},
		{
			name:         "telemetry at the possibly accepted setpoint before its effective time proves nothing",
			now:          effective.Add(2 * time.Minute),
			observations: []Telemetry{{PowerKW: 5, ObservedAt: effective.Add(-time.Second)}},
			standing:     PossiblyOperating, countedKW: 5, replaceKW: 0,
		},
		{
			name:         "telemetry at the possibly accepted setpoint after its effective time proves execution",
			now:          effective.Add(2 * time.Minute),
			observations: []Telemetry{{PowerKW: 1.9, ObservedAt: effective.Add(30 * time.Second)}, {PowerKW: 5.04, ObservedAt: effective.Add(90 * time.Second)}},
			standing:     ProvenExecuting, countedKW: 5, replaceKW: 0,
		},
		{
			name:     "expiry releases the capacity",
			now:      interval.IntervalEnd,
			standing: Expired, countedKW: 0, replaceKW: 5,
		},
		{
			name:         "expiry wins over earlier proof",
			now:          interval.IntervalEnd.Add(time.Second),
			observations: []Telemetry{{PowerKW: 5, ObservedAt: effective.Add(90 * time.Second)}},
			standing:     Expired, countedKW: 0, replaceKW: 5,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Resolve(interval, testCase.now, testCase.observations)
			want := Resolved{Standing: testCase.standing, CountedKW: testCase.countedKW}
			if got != want {
				t.Fatalf("resolved = %+v, want %+v", got, want)
			}
			if replace := ReplaceableKW(10, 5, []Resolved{got}); replace != testCase.replaceKW {
				t.Fatalf("replaceable = %v kW, want %v kW", replace, testCase.replaceKW)
			}
		})
	}
}

func TestNoOvershootReplacesOnlyTheUncoveredShortfall(t *testing.T) {
	if replace := ReplaceableKW(10, 5, nil); replace != 5 {
		t.Fatalf("replaceable without uncertain devices = %v kW, want 5 kW", replace)
	}
	if replace := ReplaceableKW(10, 12, nil); replace != 0 {
		t.Fatalf("replaceable above target = %v kW, want 0 kW", replace)
	}
	uncertain := []Resolved{
		{Standing: PossiblyOperating, CountedKW: 3},
		{Standing: Expired, CountedKW: 0},
		{Standing: ProvenExecuting, CountedKW: 1},
	}
	if replace := ReplaceableKW(10, 5, uncertain); replace != 1 {
		t.Fatalf("replaceable = %v kW, want 1 kW", replace)
	}
}
