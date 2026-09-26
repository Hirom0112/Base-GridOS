package reconciliation

import (
	"slices"
	"time"
)

type History struct {
	observations []Telemetry
}

func (history *History) Observe(observation Telemetry) bool {
	position, present := slices.BinarySearchFunc(history.observations, observation.ObservedAt, func(known Telemetry, at time.Time) int {
		return known.ObservedAt.Compare(at)
	})
	if present {
		return false
	}
	history.observations = slices.Insert(history.observations, position, observation)
	return true
}

func (history *History) Observations() []Telemetry {
	return slices.Clone(history.observations)
}

func (history *History) Delivery(measurement Measurement) Delivery {
	return Integrate(measurement, history.observations)
}
