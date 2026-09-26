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

func TestExportPreservesEveryFactKindAndProvenance(t *testing.T) {
	directory := t.TempDir()
	sink := NewLocalSink(directory)
	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
	kinds := []Kind{RawTelemetry, NormalizedTelemetry, ForecastFact, ActualFact, DispatchFact, VerificationFact, DataQualityFact}
	for _, kind := range kinds {
		record := Record{
			ID: string(kind) + ":source-1", Kind: kind,
			Provenance: Provenance{
				Class: "DERIVED", SourceID: "source-1", SourceURI: "fixture:source-1",
				ObservedAt: now, IngestedAt: now, SchemaVersion: "1",
			},
			Payload: json.RawMessage(`{"value":1}`),
		}
		if err := sink.Write(context.Background(), record); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.Open(filepath.Join(directory, "records.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	scanner := bufio.NewScanner(file)
	for _, expected := range kinds {
		if !scanner.Scan() {
			t.Fatalf("missing %s record", expected)
		}
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Kind != expected || record.Provenance.Class != "DERIVED" || record.Provenance.SourceID != "source-1" {
			t.Fatalf("record lost type or provenance: %+v", record)
		}
	}
	if scanner.Scan() {
		t.Fatal("unexpected extra record")
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestExportRejectsMissingLineageAndIdCollision(t *testing.T) {
	sink := NewLocalSink(t.TempDir())
	now := time.Now().UTC()
	record := Record{
		ID: "forecast:1", Kind: ForecastFact,
		Provenance: Provenance{Class: "DERIVED", SourceID: "load-profile", SourceURI: "fixture:load-profile", ObservedAt: now, IngestedAt: now, SchemaVersion: "1"},
		Payload:    json.RawMessage(`{"value":1}`),
	}
	if err := sink.Write(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write(context.Background(), record); err != nil {
		t.Fatalf("identical retry failed: %v", err)
	}
	record.Payload = json.RawMessage(`{"value":2}`)
	if err := sink.Write(context.Background(), record); err == nil {
		t.Fatal("id collision accepted")
	}
	record.ID = "forecast:2"
	record.Provenance.SourceID = ""
	if err := sink.Write(context.Background(), record); err == nil {
		t.Fatal("missing lineage accepted")
	}
}
