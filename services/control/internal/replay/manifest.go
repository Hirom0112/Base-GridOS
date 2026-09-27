package replay

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var eventIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type Input struct {
	EventID               string `json:"event_id"`
	Seed                  int64  `json:"seed"`
	FleetFile             string `json:"fleet_file"`
	ScenarioFile          string `json:"scenario_file,omitempty"`
	InputSnapshotID       string `json:"input_snapshot_id"`
	EligibilitySnapshotID string `json:"eligibility_snapshot_id"`
	PolicyVersion         string `json:"policy_version"`
	SolverVersion         string `json:"solver_version"`
	FallbackVersion       string `json:"fallback_version"`
	CodeVersion           string `json:"code_version"`
}

type Manifest struct {
	Input
	FleetSHA256    string `json:"fleet_sha256"`
	ScenarioSHA256 string `json:"scenario_sha256,omitempty"`
}

func Create(directory string, input Input) (result Manifest, createErr error) {
	if err := input.validate(); err != nil {
		return Manifest{}, err
	}
	fleetHash, err := fileHash(input.FleetFile)
	if err != nil {
		return Manifest{}, fmt.Errorf("hash fleet: %w", err)
	}
	scenarioHash := ""
	if input.ScenarioFile != "" {
		scenarioHash, err = fileHash(input.ScenarioFile)
		if err != nil {
			return Manifest{}, fmt.Errorf("hash scenario: %w", err)
		}
	}
	manifest := Manifest{Input: input, FleetSHA256: fleetHash, ScenarioSHA256: scenarioHash}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return Manifest{}, err
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, err
	}
	temporary, err := os.CreateTemp(directory, ".manifest-*")
	if err != nil {
		return Manifest{}, err
	}
	defer func() {
		createErr = errors.Join(createErr, os.Remove(temporary.Name()))
	}()
	_, err = temporary.Write(data)
	if err == nil {
		err = temporary.Sync()
	}
	if err = errors.Join(err, temporary.Close()); err != nil {
		return Manifest{}, err
	}
	if err = os.Link(temporary.Name(), manifestPath(directory, input.EventID)); err == nil {
		return manifest, nil
	}
	if !os.IsExist(err) {
		return Manifest{}, err
	}
	previous, err := Load(directory, input.EventID)
	if err != nil {
		return Manifest{}, err
	}
	if previous != manifest {
		return Manifest{}, errors.New("event manifest already exists with different content")
	}
	return previous, nil
}

func Load(directory, eventID string) (Manifest, error) {
	if !eventIDPattern.MatchString(eventID) {
		return Manifest{}, errors.New("invalid event id")
	}
	data, err := os.ReadFile(manifestPath(directory, eventID))
	if err != nil {
		return Manifest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return Manifest{}, errors.New("manifest has trailing data")
	}
	if err := manifest.validate(); err != nil {
		return Manifest{}, err
	}
	if manifest.EventID != eventID || !validHash(manifest.FleetSHA256) ||
		(manifest.ScenarioFile == "" && manifest.ScenarioSHA256 != "") ||
		(manifest.ScenarioFile != "" && !validHash(manifest.ScenarioSHA256)) {
		return Manifest{}, errors.New("manifest identity or hashes are invalid")
	}
	return manifest, nil
}

func (input Input) validate() error {
	if input.CodeVersion == "" || input.CodeVersion == "(devel)" {
		return errors.New("replay manifest code version required")
	}
	if !eventIDPattern.MatchString(input.EventID) || input.Seed == 0 || input.FleetFile == "" ||
		input.InputSnapshotID == "" || input.EligibilitySnapshotID == "" || input.PolicyVersion == "" ||
		input.SolverVersion == "" || input.FallbackVersion == "" {
		return errors.New("replay manifest seed or input is incomplete or unsafe")
	}
	return nil
}

func manifestPath(directory, eventID string) string {
	return filepath.Join(directory, eventID+".json")
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func validHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
