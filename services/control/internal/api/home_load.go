package api

import (
	"cmp"
	"math"
	"slices"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
)

type frozenSiteLoad struct {
	begin time.Time
	kwh   float64
}

func ApplyFrozenHomeLoads(devices map[string]safety.DeviceState, request *gridosv1.OptimizationRequest) {
	homeLoads := frozenHomeLoadsKW(request)
	for _, device := range request.GetDevices() {
		state, present := devices[device.GetDeviceId()]
		if !present {
			continue
		}
		state.HomeLoadKW = homeLoads[device.GetSiteId()]
		devices[device.GetDeviceId()] = state
	}
}

func frozenHomeLoadsKW(request *gridosv1.OptimizationRequest) map[string]float64 {
	loads := make(map[string]float64)
	if request.GetMeasurementBoundary() != gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT {
		return loads
	}
	series := make(map[string][]frozenSiteLoad)
	for _, load := range request.GetForecast().GetSiteLoads() {
		kwh := max(load.GetLoadKwh().GetValue(), load.GetLoadKwh().GetUpper())
		if math.IsInf(kwh, 0) || kwh < 0 || load.GetLoadKwh().GetLower() < 0 {
			kwh = math.NaN()
		}
		series[load.GetSiteId()] = append(series[load.GetSiteId()], frozenSiteLoad{begin: load.GetIntervalBeginTime().AsTime(), kwh: kwh})
	}
	for _, device := range request.GetDevices() {
		loads[device.GetSiteId()] = math.NaN()
		if device.GetSiteId() != "" {
			loads[device.GetSiteId()] = peakHomeLoadKW(series[device.GetSiteId()], request.GetIntervals())
		}
	}
	return loads
}

func peakHomeLoadKW(series []frozenSiteLoad, intervals []*gridosv1.OptimizationInterval) float64 {
	slices.SortFunc(series, func(left, right frozenSiteLoad) int {
		return cmp.Or(left.begin.Compare(right.begin), cmp.Compare(left.kwh, right.kwh))
	})
	peak := 0.0
	for _, interval := range intervals {
		begin := interval.GetBeginTime().AsTime()
		covering := slices.IndexFunc(series, func(load frozenSiteLoad) bool { return load.begin.After(begin) })
		if covering < 0 {
			covering = len(series)
		}
		if covering == 0 {
			return math.NaN()
		}
		peak = max(peak, series[covering-1].kwh/interval.GetEndTime().AsTime().Sub(begin).Hours())
	}
	return peak
}
