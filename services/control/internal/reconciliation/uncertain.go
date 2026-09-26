package reconciliation

import (
	"errors"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

var ErrExpiredBeforeUncertain = errors.New("command expired before its acknowledgement became uncertain")

type Command struct {
	ID          string
	SetpointKW  float64
	EffectiveAt time.Time
	ExpiresAt   time.Time
}

type Telemetry struct {
	PowerKW    float64
	ObservedAt time.Time
}

type UncertainSend struct {
	DeviceID           string
	LastConfirmed      Command
	PossiblyAccepted   Command
	MaxRampKWPerSecond float64
	Fresh              Telemetry
	DerivedAt          time.Time
	CorrelationID      string
}

func FeasibleInterval(send UncertainSend) (storage.FeasiblePowerInterval, error) {
	begin, end := send.DerivedAt, send.PossiblyAccepted.ExpiresAt
	if !end.After(begin) {
		return storage.FeasiblePowerInterval{}, ErrExpiredBeforeUncertain
	}
	reachable := send.reachableBy(end)
	lower, upper := send.PossiblyAccepted.SetpointKW, send.PossiblyAccepted.SetpointKW
	for _, candidate := range send.holdoverCandidates(begin, end) {
		lower = min(lower, reachable(candidate))
		upper = max(upper, reachable(candidate))
	}
	lower = min(lower, send.Fresh.PowerKW)
	upper = max(upper, send.Fresh.PowerKW)
	return storage.FeasiblePowerInterval{
		DeviceID:                    send.DeviceID,
		IntervalBegin:               begin,
		IntervalEnd:                 end,
		LowerKW:                     lower,
		UpperKW:                     upper,
		LastConfirmedCommandID:      send.LastConfirmed.ID,
		LastConfirmedSetpointKW:     send.LastConfirmed.SetpointKW,
		PossiblyAcceptedCommandID:   send.PossiblyAccepted.ID,
		PossiblyAcceptedSetpointKW:  send.PossiblyAccepted.SetpointKW,
		PossiblyAcceptedEffectiveAt: send.PossiblyAccepted.EffectiveAt,
		PossiblyAcceptedExpiresAt:   send.PossiblyAccepted.ExpiresAt,
		MaxRampKWPerSecond:          send.MaxRampKWPerSecond,
		FreshTelemetryPowerKW:       send.Fresh.PowerKW,
		FreshTelemetryObservedAt:    send.Fresh.ObservedAt,
		DerivedAt:                   send.DerivedAt,
		CorrelationID:               send.CorrelationID,
	}, nil
}

func (send UncertainSend) holdoverCandidates(begin, end time.Time) []float64 {
	candidates := []float64{send.LastConfirmed.SetpointKW}
	if send.LastConfirmed.ExpiresAt.Before(end) {
		candidates = append(candidates, 0)
	}
	if !send.LastConfirmed.ExpiresAt.After(begin) {
		candidates = candidates[1:]
	}
	return candidates
}

func (send UncertainSend) reachableBy(end time.Time) func(float64) float64 {
	if send.MaxRampKWPerSecond == 0 {
		return func(setpoint float64) float64 { return setpoint }
	}
	reach := send.MaxRampKWPerSecond * end.Sub(send.Fresh.ObservedAt).Seconds()
	lowest, highest := send.Fresh.PowerKW-reach, send.Fresh.PowerKW+reach
	return func(setpoint float64) float64 { return min(max(setpoint, lowest), highest) }
}
