package reconciliation

import (
	"maps"
	"math"
	"slices"
	"time"
)

const ReportingInterval = 5 * time.Minute

func VerificationInterval(window time.Duration) time.Duration {
	return max(5*time.Second, min(ReportingInterval, window/12))
}

type issuedCommand struct {
	Command
	deviceID     string
	expiry       time.Time
	acknowledged bool
}

type Event struct {
	measurement Measurement
	commands    []*issuedCommand
	byID        map[string]*issuedCommand
	histories   map[string]*History
}

func NewEvent(measurement Measurement) *Event {
	return &Event{measurement: measurement, byID: map[string]*issuedCommand{}, histories: map[string]*History{}}
}

func (event *Event) Command(deviceID string, command Command) {
	issued := &issuedCommand{Command: command, deviceID: deviceID, expiry: command.ExpiresAt}
	event.commands = append(event.commands, issued)
	event.byID[command.ID] = issued
	if _, known := event.histories[deviceID]; !known {
		event.histories[deviceID] = new(History)
	}
}

func (event *Event) Acknowledge(commandID string) bool {
	issued, known := event.byID[commandID]
	if known {
		issued.acknowledged = true
	}
	return known
}

func (event *Event) Expire(commandID string, at time.Time) bool {
	issued, known := event.byID[commandID]
	if known && at.Before(issued.expiry) {
		issued.expiry = at
	}
	return known
}

func (event *Event) Observe(deviceID string, observation Telemetry) bool {
	history, known := event.histories[deviceID]
	return known && history.Observe(observation)
}

type DeviceGap struct {
	DeviceID string
	Gap
}

type Interval struct {
	Begin            time.Time
	End              time.Time
	CommandedKWh     float64
	DeliveredKWh     float64
	TrackingErrorKWh float64
	DeliveredKW      float64
	TrackingErrorKW  float64
	Measured         time.Duration
	Expected         time.Duration
}

type Response struct {
	Latency   time.Duration
	Responded int
	Commanded int
}

type Verification struct {
	Intervals       []Interval
	CommandedKWh    float64
	AcknowledgedKWh float64
	DeliveredKWh    float64
	Measured        time.Duration
	Expected        time.Duration
	Gaps            []DeviceGap
	Response        Response
}

func (event *Event) Verify() Verification {
	verification := Verification{Response: event.response()}
	window := event.measurement
	for begin := window.Begin; begin.Before(window.End); begin = begin.Add(ReportingInterval) {
		verification.Intervals = append(verification.Intervals, event.interval(begin, earliest(begin.Add(ReportingInterval), window.End)))
	}
	for _, deviceID := range event.deviceIDs() {
		delivery := event.histories[deviceID].Delivery(window)
		verification.DeliveredKWh += delivery.DeliveredKWh
		verification.Measured += delivery.Measured
		verification.Expected += window.End.Sub(window.Begin)
		for _, gap := range delivery.Gaps {
			verification.Gaps = append(verification.Gaps, DeviceGap{DeviceID: deviceID, Gap: gap})
		}
		for _, piece := range event.pieces(deviceID, window.Begin, window.End) {
			verification.CommandedKWh += piece.energyKWh()
			if piece.command != nil && piece.command.acknowledged {
				verification.AcknowledgedKWh += piece.energyKWh()
			}
		}
	}
	return verification
}

func (event *Event) interval(begin, end time.Time) Interval {
	result := Interval{Begin: begin, End: end}
	for _, deviceID := range event.deviceIDs() {
		history := event.histories[deviceID]
		full := history.Delivery(Measurement{Begin: begin, End: end, MaxGap: event.measurement.MaxGap})
		commandedMeasuredKWh := 0.0
		for _, piece := range event.pieces(deviceID, begin, end) {
			result.CommandedKWh += piece.energyKWh()
			measured := history.Delivery(Measurement{Begin: piece.begin, End: piece.end, MaxGap: event.measurement.MaxGap}).Measured
			commandedMeasuredKWh += piece.setpointKW() * measured.Hours()
		}
		result.DeliveredKWh += full.DeliveredKWh
		result.TrackingErrorKWh += full.DeliveredKWh - commandedMeasuredKWh
		result.Measured += full.Measured
		result.Expected += end.Sub(begin)
		if full.Measured > 0 {
			result.DeliveredKW += full.DeliveredKWh / full.Measured.Hours()
			result.TrackingErrorKW += (full.DeliveredKWh - commandedMeasuredKWh) / full.Measured.Hours()
		}
	}
	return result
}

func (event *Event) response() Response {
	response := Response{Commanded: len(event.commands)}
	for _, issued := range event.commands {
		for _, observation := range event.histories[issued.deviceID].observations {
			if observation.ObservedAt.Before(issued.EffectiveAt) {
				continue
			}
			if !observation.ObservedAt.Before(issued.expiry) {
				break
			}
			if math.Abs(observation.PowerKW-issued.SetpointKW) <= ExecutionProofToleranceKW {
				response.Responded++
				response.Latency = max(response.Latency, observation.ObservedAt.Sub(issued.EffectiveAt))
				break
			}
		}
	}
	return response
}

func (event *Event) deviceIDs() []string {
	return slices.Sorted(maps.Keys(event.histories))
}

type piece struct {
	begin   time.Time
	end     time.Time
	command *issuedCommand
}

func (piece piece) setpointKW() float64 {
	if piece.command == nil {
		return 0
	}
	return piece.command.SetpointKW
}

func (piece piece) energyKWh() float64 {
	return piece.setpointKW() * piece.end.Sub(piece.begin).Hours()
}

func (event *Event) pieces(deviceID string, begin, end time.Time) []piece {
	cuts := []time.Time{begin, end}
	for _, issued := range event.commands {
		if issued.deviceID != deviceID {
			continue
		}
		for _, at := range []time.Time{issued.EffectiveAt, issued.expiry} {
			if at.After(begin) && at.Before(end) {
				cuts = append(cuts, at)
			}
		}
	}
	slices.SortFunc(cuts, time.Time.Compare)
	cuts = slices.CompactFunc(cuts, time.Time.Equal)
	pieces := make([]piece, 0, len(cuts)-1)
	for index := 0; index+1 < len(cuts); index++ {
		pieces = append(pieces, piece{begin: cuts[index], end: cuts[index+1], command: event.inForce(deviceID, cuts[index])})
	}
	return pieces
}

func (event *Event) inForce(deviceID string, at time.Time) *issuedCommand {
	for index := len(event.commands) - 1; index >= 0; index-- {
		issued := event.commands[index]
		if issued.deviceID == deviceID && !at.Before(issued.EffectiveAt) && at.Before(issued.expiry) {
			return issued
		}
	}
	return nil
}
