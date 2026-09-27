package report

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"
)

type storedReportSource struct {
	eventID   string
	data      StoredEvent
	published *EventReport
	versions  map[uint64]*EventReport
	reads     int
}

func TestBuildPublishedSelectsVersionAndRejectsMissingHistory(t *testing.T) {
	older := &EventReport{EventID: "event-history", PlanVersion: 2, RequestedMW: 1}
	latest := &EventReport{EventID: "event-history", PlanVersion: 3, RequestedMW: 2}
	source := &storedReportSource{published: latest, versions: map[uint64]*EventReport{2: older, 3: latest}}
	version := uint64(2)
	got, err := BuildPublished(context.Background(), source, "event-history", &version)
	if err != nil || got.PlanVersion != 2 || got.RequestedMW != 1 {
		t.Fatalf("historical report = %+v, error = %v", got, err)
	}
	got.RequestedMW = 99
	if older.RequestedMW != 1 {
		t.Fatal("historical report aliases published evidence")
	}
	got, err = BuildPublished(context.Background(), source, "event-history", nil)
	if err != nil || got.PlanVersion != 3 {
		t.Fatalf("latest published report = %+v, error = %v", got, err)
	}
	version = 4
	_, err = BuildPublished(context.Background(), source, "event-history", &version)
	if !errors.Is(err, ErrNotPublished) || source.reads != 0 {
		t.Fatalf("missing history used live data: %v, reads = %d", err, source.reads)
	}
}

func TestFullEventReportPreservesAccountingAndModeledEconomics(t *testing.T) {
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	data := StoredEvent{
		RequestedMW: 20, ApprovedMW: 18, CommandedMW: 17, AcknowledgedMW: 16,
		Energy:                     &EnergyTotals{RequestedMWh: 40, ApprovedMWh: 36, CommandedMWh: 34, AcknowledgedMWh: 32, DeliveredMWh: 30},
		Measurement:                &Measurement{BaselineMW: 4, BaselineMWh: 8, BaselineMethod: "matched-day", DeliveryMethod: "meter-net-export", Availability: 0.92, Confidence: 0.88},
		Delivered:                  &Delivered{DeliveredMW: 15, DeliveredMWh: 30, TrackingErrorMW: -1, ResponseLatency: 12 * time.Second},
		ReserveViolationsPrevented: 7,
		Economics:                  &ModeledEconomics{GrossValueUSD: 100, DegradationCostUSD: 12, PenaltyExposureUSD: 3},
		DataGaps:                   []DataGap{{Begin: begin, End: begin.Add(5 * time.Minute), Reason: "missing telemetry"}},
		Assumptions:                []string{"baseline uses matched day"}, Provenance: []string{"SIMULATED", "DERIVED"},
		Versions: Versions{Policy: "policy-7", Solver: "highs-1", Model: "dispatch-2", Forecast: "load-3", Availability: "reliability-1", Baseline: "matched-day-1", Economics: "value-1"},
	}
	source := &storedReportSource{data: data}
	got, err := Build(context.Background(), source, "event-full")
	if err != nil {
		t.Fatal(err)
	}
	if got.Energy == nil || *got.Energy != *data.Energy || got.Measurement == nil || *got.Measurement != *data.Measurement {
		t.Fatalf("energy or measurement missing: %+v", got)
	}
	assertModeledEconomics(t, got)
	if !reflect.DeepEqual(got.DataGaps, data.DataGaps) || !reflect.DeepEqual(got.Assumptions, data.Assumptions) || got.Versions != data.Versions {
		t.Fatalf("gaps, assumptions, or versions missing: %+v", got)
	}
	got.Energy.RequestedMWh = 0
	got.DataGaps[0].Reason = "changed"
	got.Assumptions[0] = "changed"
	if data.Energy.RequestedMWh != 40 || data.DataGaps[0].Reason != "missing telemetry" || data.Assumptions[0] != "baseline uses matched day" {
		t.Fatal("report aliases stored evidence")
	}
}

func assertModeledEconomics(t *testing.T, report EventReport) {
	t.Helper()
	if report.ReserveViolationsPrevented != 7 || report.Economics == nil || report.Economics.GrossValueUSD != 100 || report.Economics.DegradationCostUSD != 12 ||
		report.Economics.PenaltyExposureUSD != 3 || report.Economics.NetValueUSD != 85 || report.Economics.ValueKind != "modeled_estimate" {
		t.Fatalf("reserve or economics missing: %+v", report)
	}
}

func TestFullEventReportRejectsNonfiniteFinancialInput(t *testing.T) {
	source := &storedReportSource{data: StoredEvent{Economics: &ModeledEconomics{GrossValueUSD: math.NaN()}}}
	_, err := Build(context.Background(), source, "event-invalid")
	if err == nil {
		t.Fatal("nonfinite modeled value accepted")
	}
}

func TestRewardsAndConservativeMarginPreserveEvidence(t *testing.T) {
	rewards := int64(725)
	source := &storedReportSource{data: StoredEvent{
		MemberRewardsCents: &rewards,
		Margin:             &ModeledMargin{ValueUSD: -3.25, HurdleUSD: 1},
	}}
	got, err := Build(context.Background(), source, "event-rewards")
	if err != nil {
		t.Fatal(err)
	}
	if got.MemberRewardsCents == nil || *got.MemberRewardsCents != 725 || got.Margin == nil || got.Margin.ValueUSD != -3.25 || got.Margin.HurdleUSD != 1 || got.Margin.ValueKind != "modeled_estimate" {
		t.Fatalf("reward or conservative margin missing: %+v", got)
	}
	*got.MemberRewardsCents = 0
	got.Margin.ValueUSD = 0
	if *source.data.MemberRewardsCents != 725 || source.data.Margin.ValueUSD != -3.25 {
		t.Fatal("report aliases stored reward or margin")
	}
}

func (source *storedReportSource) EventReportData(_ context.Context, eventID string) (StoredEvent, error) {
	source.eventID = eventID
	source.reads++
	return source.data, nil
}

func (source *storedReportSource) StoredReport(context.Context, string) (*EventReport, error) {
	return source.published, nil
}

func (source *storedReportSource) StoredReportVersion(_ context.Context, _ string, version uint64) (*EventReport, error) {
	return source.versions[version], nil
}

func TestImmutableReportReturnsPublishedVersion(t *testing.T) {
	source := &storedReportSource{data: StoredEvent{RequestedMW: 20}}
	first, err := Build(context.Background(), source, "event-immutable")
	if err != nil {
		t.Fatal(err)
	}
	source.published = &first
	source.data.RequestedMW = 99
	second, err := Build(context.Background(), source, "event-immutable")
	if err != nil {
		t.Fatal(err)
	}
	if second.RequestedMW != 20 || source.reads != 1 {
		t.Fatalf("published report changed or raw data reread: %+v reads=%d", second, source.reads)
	}
}

func TestBuildEventReportFromStorage(t *testing.T) {
	source := &storedReportSource{data: StoredEvent{
		RequestedMW:    20,
		ApprovedMW:     18,
		CommandedMW:    17.5,
		AcknowledgedMW: 16,
		Exclusions: map[string]uint64{
			"UNDER_RESERVE": 12,
			"STALE":         3,
		},
		Provenance: []string{"simulated", "derived"},
		Versions:   Versions{Policy: "policy-7", Solver: "fallback-1", Model: "load-3"},
	}}

	got, err := Build(context.Background(), source, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if source.eventID != "event-1" {
		t.Fatalf("storage event ID = %q, want event-1", source.eventID)
	}
	want := EventReport{
		EventID:          "event-1",
		RequestedMW:      20,
		ApprovedMW:       18,
		CommandedMW:      17.5,
		AcknowledgedMW:   16,
		ExcludedByReason: map[string]uint64{"UNDER_RESERVE": 12, "STALE": 3},
		Provenance:       []string{"simulated", "derived"},
		Versions:         Versions{Policy: "policy-7", Solver: "fallback-1", Model: "load-3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report = %#v, want %#v", got, want)
	}
}

func TestBuildEventReportIncludesDelivered(t *testing.T) {
	begin := time.Date(2026, 8, 12, 23, 0, 0, 0, time.UTC)
	delivered := &Delivered{
		DeliveredMWh:    1.25,
		DeliveredMW:     2.5,
		TrackingErrorMW: -0.1,
		ResponseLatency: 45 * time.Second,
		Completeness:    0.9,
		Responded:       3,
		Commanded:       4,
		UncertainIntervals: []UncertainInterval{
			{DeviceID: "device-1", Begin: begin, End: begin.Add(30 * time.Minute), Bounds: &PowerBounds{LowerKW: 2, UpperKW: 5}},
			{DeviceID: "device-2", Begin: begin.Add(20 * time.Minute), End: begin.Add(45 * time.Minute)},
		},
	}
	source := &storedReportSource{data: StoredEvent{RequestedMW: 20, Delivered: delivered}}

	got, err := Build(context.Background(), source, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Delivered, delivered) {
		t.Fatalf("delivered = %#v, want %#v", got.Delivered, delivered)
	}
	if got.Delivered == delivered || &got.Delivered.UncertainIntervals[0] == &delivered.UncertainIntervals[0] {
		t.Fatal("report aliases the stored delivery")
	}
}

func TestBuildEventReportWithoutDeliveredStaysUnverified(t *testing.T) {
	source := &storedReportSource{data: StoredEvent{RequestedMW: 20}}
	got, err := Build(context.Background(), source, "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Delivered != nil {
		t.Fatalf("delivered = %#v, want nil before verification", got.Delivered)
	}
}
