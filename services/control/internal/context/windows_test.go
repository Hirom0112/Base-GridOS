package context

import (
	"math"
	"testing"
	"time"
)

func TestWindowsRankPriceLoadAndOutageRisk(t *testing.T) {
	begin := time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC)
	base := CandidateWindow{Begin: begin, End: begin.Add(time.Hour), PriceUSDPerMWh: 50, RegionalLoadMW: 1000, OutageRisk: 0.2, FeasibleCapacityMW: 1}
	price := base
	price.Begin = begin.Add(time.Hour)
	price.End = price.Begin.Add(time.Hour)
	price.PriceUSDPerMWh = 75
	load := base
	load.Begin = begin.Add(2 * time.Hour)
	load.End = load.Begin.Add(time.Hour)
	load.RegionalLoadMW = 1500
	risk := base
	risk.Begin = begin.Add(3 * time.Hour)
	risk.End = risk.Begin.Add(time.Hour)
	risk.OutageRisk = 0.4
	windows, err := RankWindows([]CandidateWindow{base, risk, load, price})
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 4 || windows[3].Begin != base.Begin {
		t.Fatalf("ranked windows = %#v", windows)
	}
	for index, window := range windows {
		if window.Rank != index+1 || window.ValueKind != "modeled_estimate" {
			t.Fatalf("window metadata = %#v", window)
		}
		if !window.Begin.Equal(base.Begin) && window.Score <= windows[3].Score {
			t.Fatalf("stronger signal did not improve rank: %#v", window)
		}
	}
	if windows[3].ForecastGridValueUSD != 50 {
		t.Fatalf("base modeled grid value = %v", windows[3].ForecastGridValueUSD)
	}
}

func TestWindowsRejectInvalidCandidate(t *testing.T) {
	begin := time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC)
	valid := CandidateWindow{Begin: begin, End: begin.Add(time.Hour), PriceUSDPerMWh: 50, RegionalLoadMW: 1000, OutageRisk: 0.2, FeasibleCapacityMW: 1}
	if _, err := RankWindows([]CandidateWindow{valid}); err != nil {
		t.Fatalf("positive control candidate rejected: %v", err)
	}
	invalid := valid
	invalid.PriceUSDPerMWh = math.NaN()
	if _, err := RankWindows([]CandidateWindow{invalid}); err == nil {
		t.Fatal("non-finite price accepted")
	}
	invalid = valid
	invalid.OutageRisk = 1.2
	if _, err := RankWindows([]CandidateWindow{invalid}); err == nil {
		t.Fatal("outage probability above one accepted")
	}
}
