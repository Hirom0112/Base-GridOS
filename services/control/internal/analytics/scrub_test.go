package analytics

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type capturedSink struct {
	record Record
}

func (sink *capturedSink) Write(_ context.Context, record Record) error {
	sink.record = record
	return nil
}

func TestScrubbedSink(t *testing.T) {
	sink := &capturedSink{}
	wrapped, err := NewScrubbedSink(sink, []byte("test-only-scrub-key"))
	if err != nil {
		t.Fatal(err)
	}
	record := Record{
		ID: "site-private-123", Kind: DispatchFact,
		Provenance: Provenance{
			Class: "SIMULATED", SourceID: "site-private-123", SourceURI: "travel-window-private-789",
			ObservedAt: time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC),
			IngestedAt: time.Date(2026, 8, 12, 18, 1, 0, 0, time.UTC), SchemaVersion: "v1",
		},
		Payload: json.RawMessage(`{"site_id":"site-private-123","command_credential":"credential-private-456","travel_window":"travel-window-private-789","count":5}`),
	}
	if err := wrapped.Write(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(sink.record)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"site-private-123", "credential-private-456", "travel-window-private-789"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("private value reached sink: %s", encoded)
		}
	}
	if !strings.Contains(string(encoded), `"count":5`) {
		t.Fatalf("safe aggregate count removed: %s", encoded)
	}
}
