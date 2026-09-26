//go:build bigquery

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestBigQueryRecordedInsert(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "insert_all.json"))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer fixture-token" || !bytes.Equal(bytes.TrimSpace(body), bytes.TrimSpace(expected)) {
			t.Errorf("request does not match recorded insert: %s", body)
		}
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte(`{}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fixture-token"}))
	sink := NewBigQuerySink(client, server.URL)
	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
	record := Record{
		ID: "fact:fixture:1", Kind: VerificationFact,
		Provenance: Provenance{
			Class: "SIMULATED", SourceID: "fixture", SourceURI: "scenario:fixture",
			ObservedAt: now, IngestedAt: now.Add(time.Second), SchemaVersion: "1",
		},
		Payload: json.RawMessage(`{"delivered_mwh":1.25}`),
	}
	if err := sink.Write(ctx, record); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("retry count = %d, want 2", calls.Load())
	}
}

func TestBigQueryLiveSmoke(t *testing.T) {
	if os.Getenv("GRIDOS_BIGQUERY_LIVE") != "1" {
		t.Skip("set GRIDOS_BIGQUERY_LIVE=1 with ADC and a table to run the live smoke")
	}
	t.Setenv("GRIDOS_ANALYTICS", "bigquery")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sink, err := NewSink(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := sink.Write(ctx, Record{
		ID: "smoke:" + now.Format(time.RFC3339Nano), Kind: DataQualityFact,
		Provenance: Provenance{Class: "SIMULATED", SourceID: "live-smoke", SourceURI: "scenario:smoke", ObservedAt: now, IngestedAt: now, SchemaVersion: "1"},
		Payload:    json.RawMessage(`{"status":"smoke"}`),
	}); err != nil {
		t.Fatal(err)
	}
}
