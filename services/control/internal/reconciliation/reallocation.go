package reconciliation

import (
	"math"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

const ExecutionProofToleranceKW = 0.05

type Standing string

const (
	PossiblyOperating Standing = "POSSIBLY_OPERATING"
	ProvenExecuting   Standing = "PROVEN_EXECUTING"
	Expired           Standing = "EXPIRED"
)

type Resolved struct {
	Standing  Standing
	CountedKW float64
}

func Resolve(interval storage.FeasiblePowerInterval, now time.Time, observations []Telemetry) Resolved {
	if !now.Before(interval.IntervalEnd) {
		return Resolved{Standing: Expired}
	}
	for _, observation := range observations {
		if observation.ObservedAt.Before(interval.PossiblyAcceptedEffectiveAt) {
			continue
		}
		if math.Abs(observation.PowerKW-interval.PossiblyAcceptedSetpointKW) <= ExecutionProofToleranceKW {
			return Resolved{Standing: ProvenExecuting, CountedKW: interval.PossiblyAcceptedSetpointKW}
		}
	}
	return Resolved{Standing: PossiblyOperating, CountedKW: interval.UpperKW}
}

func ReplaceableKW(targetKW, confirmedKW float64, uncertain []Resolved) float64 {
	covered := confirmedKW
	for _, device := range uncertain {
		covered += device.CountedKW
	}
	return max(0, targetKW-covered)
}
