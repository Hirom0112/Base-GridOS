package report

import (
	"context"
	"time"
)

type Versions struct {
	Policy string
	Solver string
	Model  string
}

type PowerBounds struct {
	LowerKW float64
	UpperKW float64
}

type UncertainInterval struct {
	DeviceID string
	Begin    time.Time
	End      time.Time
	Bounds   *PowerBounds
}

type Delivered struct {
	DeliveredMWh       float64
	DeliveredMW        float64
	TrackingErrorMW    float64
	ResponseLatency    time.Duration
	Completeness       float64
	Responded          int
	Commanded          int
	UncertainIntervals []UncertainInterval
}

type StoredEvent struct {
	RequestedMW    float64
	ApprovedMW     float64
	CommandedMW    float64
	AcknowledgedMW float64
	Exclusions     map[string]uint64
	Provenance     []string
	Versions       Versions
	Delivered      *Delivered
}

type Source interface {
	EventReportData(context.Context, string) (StoredEvent, error)
}

type EventReport struct {
	EventID          string
	RequestedMW      float64
	ApprovedMW       float64
	CommandedMW      float64
	AcknowledgedMW   float64
	ExcludedByReason map[string]uint64
	Provenance       []string
	Versions         Versions
	Delivered        *Delivered
}

func Build(ctx context.Context, source Source, eventID string) (EventReport, error) {
	stored, err := source.EventReportData(ctx, eventID)
	if err != nil {
		return EventReport{}, err
	}
	exclusions := make(map[string]uint64, len(stored.Exclusions))
	for reason, count := range stored.Exclusions {
		exclusions[reason] = count
	}
	return EventReport{
		EventID:          eventID,
		RequestedMW:      stored.RequestedMW,
		ApprovedMW:       stored.ApprovedMW,
		CommandedMW:      stored.CommandedMW,
		AcknowledgedMW:   stored.AcknowledgedMW,
		ExcludedByReason: exclusions,
		Provenance:       append([]string(nil), stored.Provenance...),
		Versions:         stored.Versions,
		Delivered:        cloneDelivered(stored.Delivered),
	}, nil
}

func cloneDelivered(delivered *Delivered) *Delivered {
	if delivered == nil {
		return nil
	}
	clone := *delivered
	clone.UncertainIntervals = make([]UncertainInterval, len(delivered.UncertainIntervals))
	for index, interval := range delivered.UncertainIntervals {
		clone.UncertainIntervals[index] = interval
		if interval.Bounds != nil {
			bounds := *interval.Bounds
			clone.UncertainIntervals[index].Bounds = &bounds
		}
	}
	return &clone
}
