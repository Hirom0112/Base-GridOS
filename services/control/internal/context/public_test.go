package context

import (
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
	if len(snapshot.DayAheadPrices) == 0 || len(snapshot.RealTimePrices) == 0 || len(snapshot.SystemLoads) == 0 || len(snapshot.OutageRates) == 0 || len(snapshot.Forecasts) == 0 || len(snapshot.Alerts) == 0 {
		t.Fatal("public context source missing")
	}
	price := snapshot.DayAheadPrices[0]
	if price.SettlementPoint != "HB_HOUSTON" || price.USDPerMWh != 22.87 || price.Source.Provenance != "CONFIRMED_PUBLIC" || price.Source.AsOf.IsZero() || price.Source.Age <= 0 {
		t.Fatalf("day-ahead price = %#v", price)
	}
	load := snapshot.SystemLoads[0]
	if load.Zone != "COAST" || load.MW != 16306.45 || load.Source.Provenance != "CONFIRMED_PUBLIC" || load.Source.Age <= 0 {
		t.Fatalf("system load = %#v", load)
	}
	outage := snapshot.OutageRates[0]
	if outage.County != "Travis" || math.Abs(outage.Rate-0.037664286849066676) > 1e-12 || outage.Source.Provenance != "DERIVED" || outage.Source.Age <= 0 {
		t.Fatalf("outage rate = %#v", outage)
	}
	forecast := snapshot.Forecasts[0]
	if forecast.City != "austin" || forecast.Source.Provenance != "CONFIRMED_PUBLIC" || forecast.Source.Age <= 0 || forecast.Start.IsZero() || forecast.End.IsZero() {
		t.Fatalf("weather forecast = %#v", forecast)
	}
	alert := snapshot.Alerts[0]
	if alert.City != "dallas" || alert.Event == "" || alert.Source.Provenance != "CONFIRMED_PUBLIC" || alert.Source.Age <= 0 {
		t.Fatalf("weather alert = %#v", alert)
	}
}
