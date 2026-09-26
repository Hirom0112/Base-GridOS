package report

import "context"

type Versions struct {
	Policy string
	Solver string
	Model  string
}

type StoredEvent struct {
	RequestedMW    float64
	ApprovedMW     float64
	CommandedMW    float64
	AcknowledgedMW float64
	Exclusions     map[string]uint64
	Provenance     []string
	Versions       Versions
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
	}, nil
}
