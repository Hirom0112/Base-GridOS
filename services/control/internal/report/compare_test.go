package report

import (
	"testing"
	"time"
)

func TestCompareReportsTracksNumericAndVersionChanges(t *testing.T) {
	before := EventReport{
		EventID: "event-1", PlanVersion: 1, RequestedMW: 20, ApprovedMW: 18, CommandedMW: 17, AcknowledgedMW: 16,
		Energy:      &EnergyTotals{RequestedMWh: 40, ApprovedMWh: 36, CommandedMWh: 34, AcknowledgedMWh: 32, DeliveredMWh: 30},
		Measurement: &Measurement{BaselineMW: 4, BaselineMWh: 8, Availability: 0.9, Confidence: 0.8},
		Delivered: &Delivered{DeliveredMW: 15, DeliveredMWh: 30, TrackingErrorMW: -1, ResponseLatency: time.Second, Completeness: 0.9, Responded: 3, Commanded: 4,
			UncertainIntervals: []UncertainInterval{{DeviceID: "device-1", Bounds: &PowerBounds{LowerKW: 1, UpperKW: 2}}}},
		ReserveViolationsPrevented: 7, ExcludedByReason: map[string]uint64{"RESERVE": 2},
		Economics: &ModeledEconomics{GrossValueUSD: 100, DegradationCostUSD: 12, PenaltyExposureUSD: 3, NetValueUSD: 85},
		Versions:  Versions{Policy: "policy-1", Solver: "highs-1"},
	}
	after := before
	after.PlanVersion = 2
	after.RequestedMW = 21
	after.Energy = &EnergyTotals{RequestedMWh: 41, ApprovedMWh: 36, CommandedMWh: 34, AcknowledgedMWh: 32, DeliveredMWh: 30}
	after.Measurement = &Measurement{BaselineMW: 5, BaselineMWh: 8, Availability: 0.9, Confidence: 0.8}
	after.Delivered = &Delivered{DeliveredMW: 14, DeliveredMWh: 30, TrackingErrorMW: -1, ResponseLatency: time.Second, Completeness: 0.9, Responded: 3, Commanded: 4,
		UncertainIntervals: []UncertainInterval{{DeviceID: "device-1", Bounds: &PowerBounds{LowerKW: 1, UpperKW: 3}}}}
	after.ReserveViolationsPrevented = 8
	after.ExcludedByReason = map[string]uint64{"RESERVE": 3}
	after.Economics = &ModeledEconomics{GrossValueUSD: 100, DegradationCostUSD: 12, PenaltyExposureUSD: 3, NetValueUSD: 86}
	after.Versions.Policy = "policy-2"
	differences := Compare(before, after)
	want := map[string]bool{
		"plan_version": true, "requested_mw": true, "energy.requested_mwh": true, "measurement.baseline_mw": true,
		"delivered.delivered_mw": true, "delivered.uncertain_intervals.0.upper_kw": true,
		"reserve_violations_prevented": true, "excluded_by_reason.RESERVE": true, "economics.net_value_usd": true, "versions.policy": true,
	}
	if len(differences) != len(want) {
		t.Fatalf("differences = %+v, want fields %+v", differences, want)
	}
	for _, difference := range differences {
		if !want[difference.Field] || difference.Before == difference.After {
			t.Fatalf("unexpected difference: %+v", difference)
		}
		delete(want, difference.Field)
	}
	if len(want) != 0 {
		t.Fatalf("missing differences: %+v", want)
	}
}

func TestCompareReportsTracksMarginBoundProvenance(t *testing.T) {
	before := EventReport{Margin: &ModeledMargin{ValueUSD: -1, Bound: "UPPER", PriceProvenance: "SIMULATED", UnavailableCosts: []string{"MEMBER_REWARD"}}}
	after := EventReport{Margin: &ModeledMargin{ValueUSD: -1, Bound: "UPPER", PriceProvenance: "CONFIRMED_PUBLIC", UnavailableCosts: []string{"CHARGING_ENERGY"}}}
	differences := Compare(before, after)
	if len(differences) != 2 || differences[0].Field != "margin.price_provenance" || differences[1].Field != "margin.unavailable_cost_terms.0" {
		t.Fatalf("margin bound differences=%+v", differences)
	}
}
