package report

import (
	"slices"
	"strconv"
)

type Difference struct {
	Field  string
	Before string
	After  string
}

func Compare(before, after EventReport) []Difference {
	first := comparisonValues(before)
	second := comparisonValues(after)
	fields := make([]string, 0, len(first)+len(second))
	for field := range first {
		fields = append(fields, field)
	}
	for field := range second {
		if _, found := first[field]; !found {
			fields = append(fields, field)
		}
	}
	slices.Sort(fields)
	differences := make([]Difference, 0)
	for _, field := range fields {
		if first[field] != second[field] {
			differences = append(differences, Difference{Field: field, Before: first[field], After: second[field]})
		}
	}
	return differences
}

func comparisonValues(report EventReport) map[string]string {
	values := make(map[string]string)
	float := func(field string, value float64) { values[field] = strconv.FormatFloat(value, 'g', -1, 64) }
	integer := func(field string, value uint64) { values[field] = strconv.FormatUint(value, 10) }
	signed := func(field string, value int64) { values[field] = strconv.FormatInt(value, 10) }
	float("requested_mw", report.RequestedMW)
	float("approved_mw", report.ApprovedMW)
	float("commanded_mw", report.CommandedMW)
	float("acknowledged_mw", report.AcknowledgedMW)
	integer("plan_version", report.PlanVersion)
	integer("reserve_violations_prevented", report.ReserveViolationsPrevented)
	for reason, count := range report.ExcludedByReason {
		integer("excluded_by_reason."+reason, count)
	}
	if report.Energy != nil {
		float("energy.requested_mwh", report.Energy.RequestedMWh)
		float("energy.approved_mwh", report.Energy.ApprovedMWh)
		float("energy.commanded_mwh", report.Energy.CommandedMWh)
		float("energy.acknowledged_mwh", report.Energy.AcknowledgedMWh)
		float("energy.delivered_mwh", report.Energy.DeliveredMWh)
	}
	if report.Measurement != nil {
		float("measurement.baseline_mw", report.Measurement.BaselineMW)
		float("measurement.baseline_mwh", report.Measurement.BaselineMWh)
		float("measurement.availability", report.Measurement.Availability)
		float("measurement.confidence", report.Measurement.Confidence)
	}
	if report.Delivered != nil {
		float("delivered.delivered_mw", report.Delivered.DeliveredMW)
		float("delivered.delivered_mwh", report.Delivered.DeliveredMWh)
		float("delivered.tracking_error_mw", report.Delivered.TrackingErrorMW)
		signed("delivered.response_latency_ns", int64(report.Delivered.ResponseLatency))
		float("delivered.completeness", report.Delivered.Completeness)
		signed("delivered.responded", int64(report.Delivered.Responded))
		signed("delivered.commanded", int64(report.Delivered.Commanded))
		for index, interval := range report.Delivered.UncertainIntervals {
			if interval.Bounds == nil {
				continue
			}
			prefix := "delivered.uncertain_intervals." + strconv.Itoa(index)
			float(prefix+".lower_kw", interval.Bounds.LowerKW)
			float(prefix+".upper_kw", interval.Bounds.UpperKW)
		}
	}
	if report.Economics != nil {
		float("economics.gross_value_usd", report.Economics.GrossValueUSD)
		float("economics.degradation_cost_usd", report.Economics.DegradationCostUSD)
		float("economics.penalty_exposure_usd", report.Economics.PenaltyExposureUSD)
		float("economics.net_value_usd", report.Economics.NetValueUSD)
	}
	if report.MemberRewardsCents != nil {
		signed("member_rewards_cents", *report.MemberRewardsCents)
	}
	if report.Margin != nil {
		float("margin.value_usd", report.Margin.ValueUSD)
		float("margin.hurdle_usd", report.Margin.HurdleUSD)
	}
	values["versions.policy"] = report.Versions.Policy
	values["versions.solver"] = report.Versions.Solver
	values["versions.model"] = report.Versions.Model
	values["versions.forecast"] = report.Versions.Forecast
	values["versions.availability"] = report.Versions.Availability
	values["versions.baseline"] = report.Versions.Baseline
	values["versions.economics"] = report.Versions.Economics
	return values
}
