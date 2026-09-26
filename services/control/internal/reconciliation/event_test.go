package reconciliation

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"pgregory.net/rapid"
)

func TestPropertyDeliveredNeverExceedsReceivedTelemetry(t *testing.T) {
	rapid.Check(t, func(check *rapid.T) {
		length := time.Duration(rapid.IntRange(5, 120).Draw(check, "minutes")) * time.Minute
		window := Measurement{Begin: eventStart, End: eventStart.Add(length), MaxGap: time.Duration(rapid.IntRange(10, 600).Draw(check, "max gap seconds")) * time.Second}
		event, bare := NewEvent(window), NewEvent(window)
		received := map[string]map[time.Time]float64{}
		for device := range rapid.IntRange(1, 3).Draw(check, "devices") {
			deviceID := fmt.Sprintf("device-%d", device)
			received[deviceID] = map[time.Time]float64{}
			for index := range rapid.IntRange(1, 3).Draw(check, "commands") {
				effective := window.Begin.Add(time.Duration(rapid.IntRange(0, int(length.Seconds())).Draw(check, "effective")) * time.Second)
				command := Command{
					ID:          fmt.Sprintf("%s-command-%d", deviceID, index),
					SetpointKW:  rapid.Float64Range(-5, 5).Draw(check, "setpoint"),
					EffectiveAt: effective,
					ExpiresAt:   effective.Add(time.Duration(rapid.IntRange(1, int(length.Seconds())).Draw(check, "duration")) * time.Second),
				}
				event.Command(deviceID, command)
				bare.Command(deviceID, command)
				if rapid.Bool().Draw(check, "acknowledged") {
					event.Acknowledge(command.ID)
				}
				if rapid.Bool().Draw(check, "expired") {
					event.Expire(command.ID, window.Begin.Add(time.Duration(rapid.IntRange(0, int(length.Seconds())).Draw(check, "expiry"))*time.Second))
				}
			}
			for range rapid.IntRange(0, 40).Draw(check, "observations") {
				observation := Telemetry{
					PowerKW:    rapid.Float64Range(0, 10).Draw(check, "power"),
					ObservedAt: window.Begin.Add(time.Duration(rapid.IntRange(-120, int(length.Seconds())+120).Draw(check, "observed")) * time.Second),
				}
				event.Observe(deviceID, observation)
				bare.Observe(deviceID, observation)
				received[deviceID][observation.ObservedAt] = max(received[deviceID][observation.ObservedAt], observation.PowerKW)
			}
		}
		verification := event.Verify()
		integral := 0.0
		for _, powers := range received {
			times := slices.SortedFunc(func(yield func(time.Time) bool) {
				for at := range powers {
					if !yield(at) {
						return
					}
				}
			}, time.Time.Compare)
			for index := 0; index+1 < len(times); index++ {
				begin, end := latest(times[index], window.Begin), earliest(times[index+1], window.End)
				if end.After(begin) {
					integral += powers[times[index]] * end.Sub(begin).Hours()
				}
			}
		}
		if verification.DeliveredKWh > integral+1e-9 {
			check.Fatalf("delivered %v kWh exceeds the %v kWh integral of received telemetry", verification.DeliveredKWh, integral)
		}
		if verification.DeliveredKWh != bare.Verify().DeliveredKWh {
			check.Fatalf("acknowledgements or expiries changed delivered energy: %v versus %v", verification.DeliveredKWh, bare.Verify().DeliveredKWh)
		}
		sliced := 0.0
		for _, interval := range verification.Intervals {
			sliced += interval.DeliveredKWh
		}
		if math.Abs(sliced-verification.DeliveredKWh) > 1e-9 {
			check.Fatalf("interval slices sum to %v kWh, whole window delivered %v kWh", sliced, verification.DeliveredKWh)
		}
	})
}
