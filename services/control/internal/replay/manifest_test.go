package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestWritesExactInputsPerEvent(t *testing.T) {
	dir := t.TempDir()
	fleet := filepath.Join(dir, "fleet.jsonl")
	scenario := filepath.Join(dir, "scenario.yaml")
	require.NoError(t, os.WriteFile(fleet, []byte("fleet-v1"), 0o600))
	require.NoError(t, os.WriteFile(scenario, []byte("scenario-v1"), 0o600))
	input := Input{
		EventID: "event-1", Seed: 42, FleetFile: fleet, ScenarioFile: scenario,
		InputSnapshotID: "input-1", EligibilitySnapshotID: "eligibility-1",
		PolicyVersion: "policy-1", SolverVersion: "highs-1",
		FallbackVersion: "fallback-1", CodeVersion: "abc123",
	}
	store := filepath.Join(dir, "manifests")
	first, err := Create(store, input)
	require.NoError(t, err)
	require.Equal(t, "3aa3a9c39cbd75515e3b4ab0095d6cc8f93d9987af2da84fdec163b1b8af7ced", first.FleetSHA256)
	require.NotEqual(t, first.FleetSHA256, first.ScenarioSHA256)
	require.Equal(t, input.EventID, first.EventID)
	require.Equal(t, input.Seed, first.Seed)
	require.Equal(t, input.InputSnapshotID, first.InputSnapshotID)
	require.Equal(t, input.EligibilitySnapshotID, first.EligibilitySnapshotID)
	require.Equal(t, input.PolicyVersion, first.PolicyVersion)
	require.Equal(t, input.SolverVersion, first.SolverVersion)
	require.Equal(t, input.FallbackVersion, first.FallbackVersion)
	require.Equal(t, input.CodeVersion, first.CodeVersion)
	stored, err := Load(store, input.EventID)
	require.NoError(t, err)
	require.Equal(t, first, stored)
	second, err := Create(store, input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.NoError(t, os.WriteFile(fleet, []byte("fleet-v2"), 0o600))
	_, err = Create(store, input)
	require.ErrorContains(t, err, "different content")
	stored, err = Load(store, input.EventID)
	require.NoError(t, err)
	require.Equal(t, first, stored)
}

func TestManifestRejectsIncompleteAndUnsafeInputs(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, Input{EventID: "../escape"})
	require.Error(t, err)
	_, err = Load(dir, "../escape")
	require.Error(t, err)
	_, err = Create(dir, Input{EventID: "event-1"})
	require.Error(t, err)
	_, err = os.Stat(filepath.Join(dir, "event-1.json"))
	require.True(t, os.IsNotExist(err))
}

func TestManifestRecordsAbsentScenarioAndRequiresSeed(t *testing.T) {
	directory := t.TempDir()
	fleet := filepath.Join(directory, "fleet.jsonl")
	require.NoError(t, os.WriteFile(fleet, []byte("fleet-v1"), 0o600))
	input := Input{
		EventID: "event-2", Seed: 42, FleetFile: fleet,
		InputSnapshotID: "input-2", EligibilitySnapshotID: "eligibility-2",
		PolicyVersion: "policy-1", SolverVersion: "highs", FallbackVersion: "fallback-1", CodeVersion: "abc123",
	}
	manifest, err := Create(directory, input)
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(directory, input.EventID+".json"))
	require.NoError(t, err)
	var encoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(content, &encoded))
	require.NotContains(t, encoded, "scenario_file")
	require.NotContains(t, encoded, "scenario_sha256")
	require.Empty(t, manifest.ScenarioFile)
	require.Empty(t, manifest.ScenarioSHA256)
	stored, err := Load(directory, input.EventID)
	require.NoError(t, err)
	require.Equal(t, manifest, stored)
	input.EventID = "missing-seed"
	input.Seed = 0
	_, err = Create(directory, input)
	require.ErrorContains(t, err, "seed")
	input.EventID = "unknown-build"
	input.Seed = 42
	input.CodeVersion = "(devel)"
	_, err = Create(directory, input)
	require.ErrorContains(t, err, "code version")
}
