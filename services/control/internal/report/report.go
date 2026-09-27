package report

import (
	"context"
	"errors"
	"math"
	"time"
)

type Versions struct {
	Policy       string
	Solver       string
	Model        string
	Forecast     string
	Availability string
	Baseline     string
	Economics    string
}

type EnergyTotals struct {
	RequestedMWh    float64
	ApprovedMWh     float64
	CommandedMWh    float64
	AcknowledgedMWh float64
	DeliveredMWh    float64
}

type Measurement struct {
	BaselineMW     float64
	BaselineMWh    float64
	BaselineMethod string
	DeliveryMethod string
	Availability   float64
	Confidence     float64
}

type ModeledEconomics struct {
	GrossValueUSD      float64
	DegradationCostUSD float64
	PenaltyExposureUSD float64
	NetValueUSD        float64
	ValueKind          string
}

type DataGap struct {
	Begin  time.Time
	End    time.Time
	Reason string
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
	PlanVersion                uint64
	RequestedMW                float64
	ApprovedMW                 float64
	CommandedMW                float64
	AcknowledgedMW             float64
	Exclusions                 map[string]uint64
	Provenance                 []string
	Versions                   Versions
	Delivered                  *Delivered
	Energy                     *EnergyTotals
	Measurement                *Measurement
	ReserveViolationsPrevented uint64
	Economics                  *ModeledEconomics
	DataGaps                   []DataGap
	Assumptions                []string
}

type Source interface {
	EventReportData(context.Context, string) (StoredEvent, error)
}

type EventReport struct {
	EventID                    string
	PlanVersion                uint64
	RequestedMW                float64
	ApprovedMW                 float64
	CommandedMW                float64
	AcknowledgedMW             float64
	ExcludedByReason           map[string]uint64
	Provenance                 []string
	Versions                   Versions
	Delivered                  *Delivered
	Energy                     *EnergyTotals
	Measurement                *Measurement
	ReserveViolationsPrevented uint64
	Economics                  *ModeledEconomics
	DataGaps                   []DataGap
	Assumptions                []string
}

func Build(ctx context.Context, source Source, eventID string) (EventReport, error) {
	if source == nil || eventID == "" {
		return EventReport{}, errors.New("report source and event identifier required")
	}
	stored, err := source.EventReportData(ctx, eventID)
	if err != nil {
		return EventReport{}, err
	}
	if err := validateAccounting(stored); err != nil {
		return EventReport{}, err
	}
	exclusions := make(map[string]uint64, len(stored.Exclusions))
	for reason, count := range stored.Exclusions {
		exclusions[reason] = count
	}
	report := EventReport{
		EventID:                    eventID,
		PlanVersion:                stored.PlanVersion,
		RequestedMW:                stored.RequestedMW,
		ApprovedMW:                 stored.ApprovedMW,
		CommandedMW:                stored.CommandedMW,
		AcknowledgedMW:             stored.AcknowledgedMW,
		ExcludedByReason:           exclusions,
		Provenance:                 append([]string(nil), stored.Provenance...),
		Versions:                   stored.Versions,
		Delivered:                  cloneDelivered(stored.Delivered),
		ReserveViolationsPrevented: stored.ReserveViolationsPrevented,
		DataGaps:                   append([]DataGap(nil), stored.DataGaps...),
		Assumptions:                append([]string(nil), stored.Assumptions...),
	}
	if stored.Energy != nil {
		energy := *stored.Energy
		report.Energy = &energy
	}
	if stored.Measurement != nil {
		measurement := *stored.Measurement
		report.Measurement = &measurement
	}
	if stored.Economics != nil {
		economics := *stored.Economics
		economics.NetValueUSD = economics.GrossValueUSD - economics.DegradationCostUSD - economics.PenaltyExposureUSD
		economics.ValueKind = "modeled_estimate"
		report.Economics = &economics
	}
	return report, nil
}

func validateAccounting(stored StoredEvent) error {
	if stored.Energy != nil && !finiteNonnegative(stored.Energy.RequestedMWh, stored.Energy.ApprovedMWh, stored.Energy.CommandedMWh, stored.Energy.AcknowledgedMWh, stored.Energy.DeliveredMWh) {
		return errors.New("report energy must be finite and nonnegative")
	}
	if stored.Measurement != nil {
		measurement := stored.Measurement
		if !finiteNonnegative(measurement.BaselineMW, measurement.BaselineMWh) || !unitInterval(measurement.Availability) || !unitInterval(measurement.Confidence) || measurement.BaselineMethod == "" || measurement.DeliveryMethod == "" {
			return errors.New("report measurement is incomplete or invalid")
		}
	}
	if stored.Economics != nil {
		economics := stored.Economics
		if !finite(economics.GrossValueUSD) || !finiteNonnegative(economics.DegradationCostUSD, economics.PenaltyExposureUSD) || !finite(economics.GrossValueUSD-economics.DegradationCostUSD-economics.PenaltyExposureUSD) {
			return errors.New("report modeled economics must be finite")
		}
	}
	for _, gap := range stored.DataGaps {
		if gap.Reason == "" || gap.Begin.IsZero() || !gap.End.After(gap.Begin) {
			return errors.New("report data gap is invalid")
		}
	}
	return nil
}

func finiteNonnegative(values ...float64) bool {
	for _, value := range values {
		if !finite(value) || value < 0 {
			return false
		}
	}
	return true
}

func unitInterval(value float64) bool {
	return finite(value) && value >= 0 && value <= 1
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
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
