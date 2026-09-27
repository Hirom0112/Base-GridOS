package telemetry

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"
)

type Profiles map[string][7][96]float64

var centralLocation, centralLocationError = time.LoadLocation("America/Chicago")

func ReadProfiles(path string) (profiles Profiles, err error) {
	if centralLocationError != nil {
		return nil, centralLocationError
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if err := validateProfileHeader(header); err != nil {
		return nil, err
	}
	profiles = make(Profiles)
	seen := make(map[string][7]bool)
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name, weekday, values, err := parseProfileRow(row)
		if err != nil {
			return nil, err
		}
		used := seen[name]
		if used[weekday] {
			return nil, fmt.Errorf("duplicate load profile weekday %s %d", name, weekday)
		}
		profile := profiles[name]
		profile[weekday] = values
		profiles[name] = profile
		used[weekday] = true
		seen[name] = used
	}
	if len(profiles) == 0 {
		return nil, errors.New("load profiles are empty")
	}
	for name, days := range seen {
		for weekday, found := range days {
			if !found {
				return nil, fmt.Errorf("load profile %s missing weekday %d", name, weekday)
			}
		}
	}
	return profiles, nil
}

func validateProfileHeader(header []string) error {
	if len(header) < 98 || header[0] != "PType_WZ" || header[1] != "Date" {
		return errors.New("invalid load profile header")
	}
	for slot := 0; slot < 96; slot++ {
		if header[slot+2] != fmt.Sprintf("int_kWh%d", slot+1) {
			return errors.New("invalid load profile interval header")
		}
	}
	return nil
}

func parseProfileRow(row []string) (string, int, [96]float64, error) {
	if len(row) < 98 || row[0] == "" {
		return "", 0, [96]float64{}, errors.New("invalid load profile row")
	}
	day, err := time.ParseInLocation("2006-01-02 15:04:05", row[1], centralLocation)
	if err != nil {
		return "", 0, [96]float64{}, err
	}
	var values [96]float64
	for slot := range values {
		value, err := strconv.ParseFloat(row[slot+2], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return "", 0, [96]float64{}, fmt.Errorf("invalid load profile value %s %d", row[0], slot)
		}
		values[slot] = value
	}
	return row[0], int(day.Weekday()), values, nil
}

func (profiles Profiles) LoadKW(name string, sourceTime time.Time) (float64, error) {
	profile, found := profiles[name]
	if !found || sourceTime.IsZero() || centralLocationError != nil {
		return 0, errors.New("load profile and source time required")
	}
	local := sourceTime.In(centralLocation)
	value := profile[int(local.Weekday())][local.Hour()*4+local.Minute()/15] * 4
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, errors.New("invalid load profile value")
	}
	return value, nil
}
