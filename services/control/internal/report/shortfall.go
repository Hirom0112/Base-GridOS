package report

import (
	"errors"
	"slices"
	"time"
)

type PlannedShortfall struct {
	Begin       time.Time `json:"begin"`
	End         time.Time `json:"end"`
	RequestedKW float64   `json:"requested_kw"`
	FeasibleKW  float64   `json:"feasible_kw"`
	ShortfallKW float64   `json:"shortfall_kw"`
	Reasons     []string  `json:"reasons"`
}

type DeliveryShortfall struct {
	Begin                time.Time `json:"begin"`
	End                  time.Time `json:"end"`
	RequestedKWh         float64   `json:"requested_kwh"`
	MeasuredDeliveredKWh *float64  `json:"measured_delivered_kwh,omitempty"`
	ShortfallKWh         *float64  `json:"shortfall_kwh,omitempty"`
	Coverage             float64   `json:"coverage"`
	ValueKind            string    `json:"value_kind"`
}

func clonePlannedShortfalls(values []PlannedShortfall) []PlannedShortfall {
	cloned := slices.Clone(values)
	for index := range cloned {
		cloned[index].Reasons = slices.Clone(values[index].Reasons)
	}
	return cloned
}

func cloneDeliveryShortfalls(values []DeliveryShortfall) []DeliveryShortfall {
	cloned := slices.Clone(values)
	for index := range cloned {
		if values[index].MeasuredDeliveredKWh != nil {
			value := *values[index].MeasuredDeliveredKWh
			cloned[index].MeasuredDeliveredKWh = &value
		}
		if values[index].ShortfallKWh != nil {
			value := *values[index].ShortfallKWh
			cloned[index].ShortfallKWh = &value
		}
	}
	return cloned
}

func validatePlannedShortfalls(values []PlannedShortfall) error {
	for _, value := range values {
		if value.Begin.IsZero() || !value.End.After(value.Begin) || !finiteNonnegative(value.RequestedKW, value.FeasibleKW, value.ShortfallKW) {
			return errors.New("planned shortfall is invalid")
		}
	}
	return nil
}

func validateDeliveryShortfalls(values []DeliveryShortfall) error {
	for _, value := range values {
		if value.Begin.IsZero() || !value.End.After(value.Begin) || !finiteNonnegative(value.RequestedKWh) || !unitInterval(value.Coverage) {
			return errors.New("delivery shortfall interval is invalid")
		}
		if value.ValueKind == "UNKNOWN" && value.MeasuredDeliveredKWh == nil && value.ShortfallKWh == nil {
			continue
		}
		if value.ValueKind != "MEASURED" || value.Coverage == 0 || value.MeasuredDeliveredKWh == nil || value.ShortfallKWh == nil || !finite(*value.MeasuredDeliveredKWh) || !finite(*value.ShortfallKWh) {
			return errors.New("delivery shortfall measurement is invalid")
		}
	}
	return nil
}
