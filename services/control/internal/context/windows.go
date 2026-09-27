package context

import (
	"errors"
	"math"
	"slices"
	"time"
)

type CandidateWindow struct {
	Begin              time.Time
	End                time.Time
	PriceUSDPerMWh     float64
	RegionalLoadMW     float64
	OutageRisk         float64
	FeasibleCapacityMW float64
	PriceSource        Source
	LoadSource         Source
	RiskSource         Source
}

type DispatchWindow struct {
	CandidateWindow
	Rank                 int
	Score                float64
	ForecastGridValueUSD float64
	ValueKind            string
}

func RankWindows(candidates []CandidateWindow) ([]DispatchWindow, error) {
	if len(candidates) == 0 {
		return []DispatchWindow{}, nil
	}
	priceMin, priceMax := candidates[0].PriceUSDPerMWh, candidates[0].PriceUSDPerMWh
	loadMin, loadMax := candidates[0].RegionalLoadMW, candidates[0].RegionalLoadMW
	riskMin, riskMax := candidates[0].OutageRisk, candidates[0].OutageRisk
	for _, candidate := range candidates {
		if err := validateCandidate(candidate); err != nil {
			return nil, err
		}
		priceMin, priceMax = min(priceMin, candidate.PriceUSDPerMWh), max(priceMax, candidate.PriceUSDPerMWh)
		loadMin, loadMax = min(loadMin, candidate.RegionalLoadMW), max(loadMax, candidate.RegionalLoadMW)
		riskMin, riskMax = min(riskMin, candidate.OutageRisk), max(riskMax, candidate.OutageRisk)
	}
	windows := make([]DispatchWindow, 0, len(candidates))
	for _, candidate := range candidates {
		value := candidate.PriceUSDPerMWh * candidate.FeasibleCapacityMW * candidate.End.Sub(candidate.Begin).Hours()
		score := normalize(candidate.PriceUSDPerMWh, priceMin, priceMax) + normalize(candidate.RegionalLoadMW, loadMin, loadMax) + normalize(candidate.OutageRisk, riskMin, riskMax)
		if math.IsNaN(value) || math.IsInf(value, 0) || math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errors.New("modeled grid value or rank is not finite")
		}
		windows = append(windows, DispatchWindow{
			CandidateWindow:      candidate,
			Score:                score,
			ForecastGridValueUSD: value,
			ValueKind:            "modeled_estimate",
		})
	}
	slices.SortFunc(windows, func(left, right DispatchWindow) int {
		if left.Score > right.Score {
			return -1
		}
		if left.Score < right.Score {
			return 1
		}
		return left.Begin.Compare(right.Begin)
	})
	for index := range windows {
		windows[index].Rank = index + 1
	}
	return windows, nil
}

func validateCandidate(candidate CandidateWindow) error {
	if candidate.Begin.IsZero() || !candidate.Begin.Before(candidate.End) {
		return errors.New("dispatch window interval is invalid")
	}
	for _, value := range []float64{candidate.PriceUSDPerMWh, candidate.RegionalLoadMW, candidate.OutageRisk, candidate.FeasibleCapacityMW} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("dispatch window signal is not finite")
		}
	}
	if candidate.RegionalLoadMW < 0 || candidate.OutageRisk < 0 || candidate.OutageRisk > 1 || candidate.FeasibleCapacityMW < 0 {
		return errors.New("dispatch window signal is out of range")
	}
	return nil
}

func normalize(value, low, high float64) float64 {
	if high == low {
		return 0
	}
	return (value - low) / (high - low)
}
