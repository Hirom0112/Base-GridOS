package scenario

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/failures"
)

type fleetDevice struct {
	DeviceID string `json:"device_id"`
}

type telemetryIdentity struct {
	DeviceID      string `json:"device_id"`
	ObservationID string `json:"observation_id"`
	Sequence      uint64 `json:"sequence"`
	SourceTime    string `json:"source_time"`
}

func TelemetryHashes(path string) ([]string, error) {
	scenario, err := failures.LoadScenario(path)
	if err != nil {
		return nil, err
	}
	deviceIDs, err := loadDeviceIDs(scenario.FleetPath)
	if err != nil {
		return nil, err
	}
	if len(deviceIDs) != scenario.FleetSize {
		return nil, fmt.Errorf("fleet size is %d, scenario requires %d", len(deviceIDs), scenario.FleetSize)
	}
	hashes := make([]string, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		identitySeed := sha256.Sum256([]byte(fmt.Sprintf("%d|%s", scenario.Seed, deviceID)))
		payload, err := json.Marshal(telemetryIdentity{
			DeviceID: deviceID, ObservationID: hex.EncodeToString(identitySeed[:16]),
			Sequence: 1, SourceTime: scenario.Start.Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(payload)
		hashes = append(hashes, hex.EncodeToString(digest[:]))
	}
	return hashes, nil
}

func loadDeviceIDs(path string) (deviceIDs []string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var device fleetDevice
		if err := json.Unmarshal(scanner.Bytes(), &device); err != nil {
			return nil, err
		}
		if device.DeviceID == "" {
			return nil, errors.New("fleet device identifier is required")
		}
		if _, exists := seen[device.DeviceID]; exists {
			return nil, fmt.Errorf("duplicate fleet device %q", device.DeviceID)
		}
		seen[device.DeviceID] = struct{}{}
		deviceIDs = append(deviceIDs, device.DeviceID)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(deviceIDs)
	return deviceIDs, nil
}
