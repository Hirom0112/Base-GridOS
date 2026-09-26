package reconciliation

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

var deadline = time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC)

func lostAcknowledgement() UncertainSend {
	return UncertainSend{
		DeviceID:           "device-1",
		LastConfirmed:      Command{ID: "command-1", SetpointKW: 2, EffectiveAt: deadline.Add(-10 * time.Minute), ExpiresAt: deadline.Add(50 * time.Minute)},
		PossiblyAccepted:   Command{ID: "command-2", SetpointKW: 5, EffectiveAt: deadline, ExpiresAt: deadline.Add(30 * time.Minute)},
		MaxRampKWPerSecond: 0.01,
		Fresh:              Telemetry{PowerKW: 2, ObservedAt: deadline.Add(-5 * time.Second)},
		DerivedAt:          deadline,
		CorrelationID:      "correlation-1",
	}
}

func TestUncertainSendDerivesFeasibleInterval(t *testing.T) {
	send := lostAcknowledgement()
	got, err := FeasibleInterval(send)
	if err != nil {
		t.Fatal(err)
	}
	want := storage.FeasiblePowerInterval{
		DeviceID:                    "device-1",
		IntervalBegin:               deadline,
		IntervalEnd:                 deadline.Add(30 * time.Minute),
		LowerKW:                     2,
		UpperKW:                     5,
		LastConfirmedCommandID:      "command-1",
		LastConfirmedSetpointKW:     2,
		PossiblyAcceptedCommandID:   "command-2",
		PossiblyAcceptedSetpointKW:  5,
		PossiblyAcceptedEffectiveAt: deadline,
		PossiblyAcceptedExpiresAt:   deadline.Add(30 * time.Minute),
		MaxRampKWPerSecond:          0.01,
		FreshTelemetryPowerKW:       2,
		FreshTelemetryObservedAt:    deadline.Add(-5 * time.Second),
		DerivedAt:                   deadline,
		CorrelationID:               "correlation-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("interval = %#v, want %#v", got, want)
	}
}

func TestUncertainSendBoundsFollowRampAndExpiry(t *testing.T) {
	var ramp, secondsToExpiry float64 = 0.0001, 1805
	cases := []struct {
		name         string
		adjust       func(*UncertainSend)
		lower, upper float64
	}{
		{
			name: "charging last confirmed keeps discharge setpoint reachable",
			adjust: func(send *UncertainSend) {
				send.LastConfirmed.SetpointKW = -4
				send.Fresh.PowerKW = -4
			},
			lower: -4, upper: 5,
		},
		{
			name: "possibly accepted charge setpoint bounds the lower side",
			adjust: func(send *UncertainSend) {
				send.PossiblyAccepted.SetpointKW = -3
			},
			lower: -3, upper: 2,
		},
		{
			name: "last confirmed expiry inside the window admits zero output",
			adjust: func(send *UncertainSend) {
				send.LastConfirmed.ExpiresAt = deadline.Add(10 * time.Minute)
			},
			lower: 0, upper: 5,
		},
		{
			name: "ramp reach from fresh telemetry clips last confirmed and zero but never the possibly accepted setpoint",
			adjust: func(send *UncertainSend) {
				send.LastConfirmed.ExpiresAt = deadline.Add(10 * time.Minute)
				send.MaxRampKWPerSecond = ramp
				send.Fresh.PowerKW = 1
			},
			lower: 1 - ramp*secondsToExpiry, upper: 5,
		},
		{
			name: "telemetry outside every setpoint widens the interval",
			adjust: func(send *UncertainSend) {
				send.Fresh.PowerKW = 6.5
			},
			lower: 2, upper: 6.5,
		},
		{
			name: "unknown ramp applies no clipping",
			adjust: func(send *UncertainSend) {
				send.LastConfirmed.ExpiresAt = deadline.Add(10 * time.Minute)
				send.MaxRampKWPerSecond = 0
				send.Fresh.PowerKW = 1
			},
			lower: 0, upper: 5,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			send := lostAcknowledgement()
			testCase.adjust(&send)
			got, err := FeasibleInterval(send)
			if err != nil {
				t.Fatal(err)
			}
			if got.LowerKW != testCase.lower || got.UpperKW != testCase.upper {
				t.Fatalf("bounds = [%v, %v], want [%v, %v]", got.LowerKW, got.UpperKW, testCase.lower, testCase.upper)
			}
			possible := send.PossiblyAccepted.SetpointKW
			if got.UpperKW < possible || got.LowerKW > possible {
				t.Fatalf("bounds [%v, %v] exclude the possibly executing setpoint %v", got.LowerKW, got.UpperKW, possible)
			}
		})
	}
}

func TestUncertainSendAfterExpiryIsRejected(t *testing.T) {
	send := lostAcknowledgement()
	send.DerivedAt = send.PossiblyAccepted.ExpiresAt
	_, err := FeasibleInterval(send)
	if !errors.Is(err, ErrExpiredBeforeUncertain) {
		t.Fatalf("error = %v, want ErrExpiredBeforeUncertain", err)
	}
}
