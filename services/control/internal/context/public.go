package context

import (
	"errors"
	"path/filepath"
	"time"
)

type Source struct {
	Provenance string
	AsOf       time.Time
	Age        time.Duration
}

type Price struct {
	At              time.Time
	SettlementPoint string
	USDPerMWh       float64
	Source          Source
}

type SystemLoad struct {
	At     time.Time
	Zone   string
	MW     float64
	Source Source
}

type OutageRate struct {
	County string
	Month  string
	Rate   float64
	Source Source
}

type Forecast struct {
	City         string
	Start        time.Time
	End          time.Time
	TemperatureF int
	Summary      string
	Source       Source
}

type Alert struct {
	City      string
	Event     string
	Severity  string
	Effective time.Time
	Expires   time.Time
	Source    Source
}

type Snapshot struct {
	DayAheadPrices []Price
	RealTimePrices []Price
	SystemLoads    []SystemLoad
	OutageRates    []OutageRate
	Forecasts      []Forecast
	Alerts         []Alert
}

func LoadPublic(root string, now time.Time) (Snapshot, error) {
	if root == "" || now.IsZero() {
		return Snapshot{}, errors.New("public fixture root and current time are required")
	}
	var snapshot Snapshot
	var err error
	snapshot.DayAheadPrices, err = loadDayAhead(filepath.Join(root, "ercot-prices", "dam-spp-week.csv"), now)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.RealTimePrices, err = loadRealTime(filepath.Join(root, "ercot-prices", "rtm-spp-week.csv"), now)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.SystemLoads, err = loadSystemLoads(filepath.Join(root, "system-load", "ercot-system-load-merged.csv"), now)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.OutageRates, err = loadOutageRates(filepath.Join(root, "outages", "outage-rates-2023-01.csv"), now)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Forecasts, snapshot.Alerts, err = loadWeather(filepath.Join(root, "weather"), now)
	return snapshot, err
}

func source(provenance string, asOf, now time.Time) (Source, error) {
	if asOf.IsZero() || asOf.After(now) {
		return Source{}, errors.New("public source timestamp is missing or in the future")
	}
	if provenance != "CONFIRMED_PUBLIC" && provenance != "DERIVED" {
		return Source{}, errors.New("public source provenance is invalid")
	}
	return Source{Provenance: provenance, AsOf: asOf, Age: now.Sub(asOf)}, nil
}
