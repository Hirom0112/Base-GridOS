package analytics

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalSinkPersistsAndDeduplicatesRecords(t *testing.T) {
	directory := t.TempDir()
	sink := NewLocalSink(directory)
	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
	seed := int64(20260926)
	record := Record{
		ID:   "observation:device-1:7",
		Kind: RawTelemetry,
		Provenance: Provenance{
			Class:          "SIMULATED",
			SourceID:       "austin-5000",
			SourceURI:      "testdata/fleets/austin-5000.jsonl",
			ObservedAt:     now,
			IngestedAt:     now.Add(time.Second),
			SchemaVersion:  "1",
			SimulationSeed: &seed,
		},
		Payload: json.RawMessage(`{"power_kw":2.5}`),
	}
	for range 2 {
		if err := sink.Write(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.Open(filepath.Join(directory, "records.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("record missing")
	}
	var stored Record
	if err := json.Unmarshal(scanner.Bytes(), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.ID != record.ID || stored.Provenance.Class != "SIMULATED" || string(stored.Payload) != string(record.Payload) {
		t.Fatalf("stored record changed: %+v", stored)
	}
	if scanner.Scan() {
		t.Fatal("duplicate record appended")
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
