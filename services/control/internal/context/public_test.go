package context

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadPublicContextProvenanceFreshness(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	root := filepath.Join("..", "..", "..", "..", "testdata", "fixtures", "public")
	snapshot, err := LoadPublic(root, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{len(snapshot.DayAheadPrices), len(snapshot.RealTimePrices), len(snapshot.SystemLoads), len(snapshot.OutageRates), len(snapshot.Forecasts), len(snapshot.Alerts)} {
		if count == 0 {
			t.Fatal("public context source missing")
		}
	}
	price := snapshot.DayAheadPrices[0]
	assertSource(t, price.Source, "CONFIRMED_PUBLIC")
	if price.SettlementPoint != "HB_HOUSTON" || price.USDPerMWh != 22.87 {
		t.Fatalf("day-ahead price = %#v", price)
	}
	load := snapshot.SystemLoads[0]
	assertSource(t, load.Source, "CONFIRMED_PUBLIC")
	if load.Zone != "COAST" || load.MW != 16306.45 {
		t.Fatalf("system load = %#v", load)
	}
	outage := snapshot.OutageRates[0]
	assertSource(t, outage.Source, "DERIVED")
	if outage.County != "Travis" || math.Abs(outage.Rate-0.037664286849066676) > 1e-12 {
		t.Fatalf("outage rate = %#v", outage)
	}
	forecast := snapshot.Forecasts[0]
	assertSource(t, forecast.Source, "CONFIRMED_PUBLIC")
	if forecast.City != "austin" || forecast.Start.IsZero() || forecast.End.IsZero() {
		t.Fatalf("weather forecast = %#v", forecast)
	}
	alert := snapshot.Alerts[0]
	assertSource(t, alert.Source, "CONFIRMED_PUBLIC")
	if alert.City != "dallas" || alert.Event == "" {
		t.Fatalf("weather alert = %#v", alert)
	}
}

func TestWeatherAlertCarriesNWSAreaEvidence(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	root := filepath.Join("..", "..", "..", "..", "testdata", "fixtures", "public")
	snapshot, err := LoadPublic(root, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, alert := range snapshot.Alerts {
		if alert.City == "houston" {
			encoded, err := json.Marshal(alert)
			if err != nil {
				t.Fatal(err)
			}
			var evidence struct {
				ID              string
				AreaDescription string
				UGC             []string
				SAME            []string
			}
			if err := json.Unmarshal(encoded, &evidence); err != nil {
				t.Fatal(err)
			}
			if evidence.ID == "" || evidence.AreaDescription == "" || len(evidence.UGC) == 0 || len(evidence.SAME) == 0 {
				t.Fatalf("NWS area evidence missing: %+v", alert)
			}
			return
		}
	}
	t.Fatal("Houston alert fixture missing")
}

func assertSource(t *testing.T, actual Source, provenance string) {
	t.Helper()
	if actual.Provenance != provenance || actual.AsOf.IsZero() || actual.Age <= 0 {
		t.Fatalf("public source = %#v", actual)
	}
}

func TestLoadPublicRejectsMissingAndFutureSources(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	if _, err := LoadPublic(t.TempDir(), now); err == nil {
		t.Fatal("missing public source accepted")
	}
	root := filepath.Join("..", "..", "..", "..", "testdata", "fixtures", "public")
	if _, err := LoadPublic(root, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("future public source accepted")
	}
}
