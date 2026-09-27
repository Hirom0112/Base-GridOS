package context

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func loadWeather(root string, now time.Time) ([]Forecast, []Alert, error) {
	forecasts := make([]Forecast, 0)
	alerts := make([]Alert, 0)
	for _, city := range []string{"austin", "dallas", "houston", "san_antonio"} {
		cityForecasts, err := loadForecast(filepath.Join(root, city+"_forecast.json"), city, now)
		if err != nil {
			return nil, nil, err
		}
		cityAlerts, err := loadAlerts(filepath.Join(root, city+"_alerts.json"), city, now)
		if err != nil {
			return nil, nil, err
		}
		forecasts = append(forecasts, cityForecasts...)
		alerts = append(alerts, cityAlerts...)
	}
	return forecasts, alerts, nil
}

func loadForecast(path, city string, now time.Time) ([]Forecast, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Properties struct {
			GeneratedAt string `json:"generatedAt"`
			Periods     []struct {
				StartTime       string `json:"startTime"`
				EndTime         string `json:"endTime"`
				Temperature     int    `json:"temperature"`
				TemperatureUnit string `json:"temperatureUnit"`
				ShortForecast   string `json:"shortForecast"`
			} `json:"periods"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	issuedAt, err := time.Parse(time.RFC3339, document.Properties.GeneratedAt)
	if err != nil {
		return nil, err
	}
	stamp, err := source("CONFIRMED_PUBLIC", issuedAt, now)
	if err != nil {
		return nil, err
	}
	if len(document.Properties.Periods) == 0 {
		return nil, errors.New("weather forecast periods are missing")
	}
	forecasts := make([]Forecast, 0, len(document.Properties.Periods))
	for _, period := range document.Properties.Periods {
		start, startErr := time.Parse(time.RFC3339, period.StartTime)
		end, endErr := time.Parse(time.RFC3339, period.EndTime)
		if startErr != nil || endErr != nil || !start.Before(end) || period.TemperatureUnit != "F" || period.ShortForecast == "" {
			return nil, errors.New("invalid weather forecast period")
		}
		forecasts = append(forecasts, Forecast{City: city, Start: start, End: end, TemperatureF: period.Temperature, Summary: period.ShortForecast, Source: stamp})
	}
	return forecasts, nil
}

func loadAlerts(path, city string, now time.Time) ([]Alert, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Features []struct {
			Properties struct {
				Sent      string `json:"sent"`
				Effective string `json:"effective"`
				Expires   string `json:"expires"`
				Event     string `json:"event"`
				Severity  string `json:"severity"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	alerts := make([]Alert, 0, len(document.Features))
	for _, feature := range document.Features {
		properties := feature.Properties
		sent, sentErr := time.Parse(time.RFC3339, properties.Sent)
		effective, effectiveErr := time.Parse(time.RFC3339, properties.Effective)
		expires, expiresErr := time.Parse(time.RFC3339, properties.Expires)
		if sentErr != nil || effectiveErr != nil || expiresErr != nil || !effective.Before(expires) || properties.Event == "" {
			return nil, errors.New("invalid weather alert")
		}
		stamp, err := source("CONFIRMED_PUBLIC", sent, now)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, Alert{City: city, Event: properties.Event, Severity: properties.Severity, Effective: effective, Expires: expires, Source: stamp})
	}
	return alerts, nil
}
