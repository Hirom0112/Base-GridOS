package context

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

var central, centralError = time.LoadLocation("America/Chicago")

func readCSV(path string, header []string, add func([]string) error) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	reader := csv.NewReader(file)
	actual, err := reader.Read()
	if err != nil {
		return err
	}
	if !slices.Equal(actual, header) {
		return fmt.Errorf("unexpected CSV header in %s", path)
	}
	count := 0
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
		if err := add(row); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("empty CSV %s", path)
	}
	return nil
}

func parseDateHour(date, hour string) (time.Time, error) {
	if centralError != nil {
		return time.Time{}, centralError
	}
	day, err := time.ParseInLocation("01/02/2006", date, central)
	if err != nil {
		return time.Time{}, err
	}
	parts := strings.Split(hour, ":")
	if len(parts) != 2 || parts[1] != "00" {
		return time.Time{}, errors.New("invalid hour ending")
	}
	hourNumber, err := strconv.Atoi(parts[0])
	if err != nil || hourNumber < 1 || hourNumber > 24 {
		return time.Time{}, errors.New("invalid hour ending")
	}
	return day.Add(time.Duration(hourNumber) * time.Hour), nil
}

func parseFinite(value string) (float64, error) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, errors.New("invalid numeric public value")
	}
	return number, nil
}

func loadDayAhead(path string, now time.Time) ([]Price, error) {
	prices := make([]Price, 0)
	err := readCSV(path, []string{"Delivery Date", "Hour Ending", "Repeated Hour Flag", "Settlement Point", "Settlement Point Price"}, func(row []string) error {
		at, err := parseDateHour(row[0], row[1])
		if err != nil {
			return err
		}
		value, err := parseFinite(row[4])
		if err != nil {
			return err
		}
		stamp, err := source("CONFIRMED_PUBLIC", at, now)
		if err != nil {
			return err
		}
		if row[3] == "" || (row[2] != "N" && row[2] != "Y") {
			return errors.New("invalid day-ahead settlement point")
		}
		prices = append(prices, Price{At: at, SettlementPoint: row[3], USDPerMWh: value, Source: stamp})
		return nil
	})
	return prices, err
}

func loadRealTime(path string, now time.Time) ([]Price, error) {
	prices := make([]Price, 0)
	err := readCSV(path, []string{"Delivery Date", "Delivery Hour", "Delivery Interval", "Repeated Hour Flag", "Settlement Point Name", "Settlement Point Type", "Settlement Point Price"}, func(row []string) error {
		day, err := time.ParseInLocation("01/02/2006", row[0], central)
		if err != nil {
			return err
		}
		hour, hourErr := strconv.Atoi(row[1])
		interval, intervalErr := strconv.Atoi(row[2])
		if hourErr != nil || intervalErr != nil || hour < 1 || hour > 24 || interval < 1 || interval > 4 || row[4] == "" {
			return errors.New("invalid real-time interval")
		}
		if row[5] == "LZEW" {
			return nil
		}
		at := day.Add(time.Duration(hour-1)*time.Hour + time.Duration(interval)*15*time.Minute)
		value, err := parseFinite(row[6])
		if err != nil {
			return err
		}
		stamp, err := source("CONFIRMED_PUBLIC", at, now)
		if err != nil {
			return err
		}
		prices = append(prices, Price{At: at, SettlementPoint: row[4], USDPerMWh: value, Source: stamp})
		return nil
	})
	return prices, err
}

func loadSystemLoads(path string, now time.Time) ([]SystemLoad, error) {
	header := []string{"OperDay", "HourEnding", "COAST", "EAST", "FAR_WEST", "NORTH", "NORTH_C", "SOUTHERN", "SOUTH_C", "WEST", "TOTAL", "DSTFlag"}
	loads := make([]SystemLoad, 0)
	err := readCSV(path, header, func(row []string) error {
		at, err := parseDateHour(row[0], row[1])
		if err != nil {
			return err
		}
		stamp, err := source("CONFIRMED_PUBLIC", at, now)
		if err != nil {
			return err
		}
		for index, zone := range header[2:11] {
			mw, err := parseFinite(row[index+2])
			if err != nil || mw < 0 {
				return errors.New("invalid regional system load")
			}
			loads = append(loads, SystemLoad{At: at, Zone: zone, MW: mw, Source: stamp})
		}
		return nil
	})
	return loads, err
}

func loadOutageRates(path string, now time.Time) ([]OutageRate, error) {
	rates := make([]OutageRate, 0)
	err := readCSV(path, []string{"county", "month", "outage_rate", "provenance"}, func(row []string) error {
		month, err := time.Parse("2006-01", row[1])
		if err != nil {
			return err
		}
		rate, err := parseFinite(row[2])
		if err != nil || rate < 0 || rate > 1 || row[0] == "" {
			return errors.New("invalid outage rate")
		}
		stamp, err := source(row[3], month.AddDate(0, 1, 0), now)
		if err != nil {
			return err
		}
		rates = append(rates, OutageRate{County: row[0], Month: row[1], Rate: rate, Source: stamp})
		return nil
	})
	return rates, err
}
