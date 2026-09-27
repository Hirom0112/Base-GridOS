package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBigQuerySinkSendsIdempotentInsert(t *testing.T) {
	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
	record := Record{
		ID:   "dispatch:event-1:verified",
		Kind: DispatchFact,
		Provenance: Provenance{
			Class: "SIMULATED", SourceID: "event-1", SourceURI: "scenario:austin",
			ObservedAt: now, IngestedAt: now, SchemaVersion: "1",
		},
		Payload: json.RawMessage(`{"delivered_mwh":1.25}`),
	}
	var received struct {
		Rows []struct {
			InsertID string `json:"insertId"`
			JSON     Record `json:"json"`
		} `json:"rows"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/insertAll" {
			t.Errorf("wrong request: %s %s", request.Method, request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte(`{}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	sink := NewBigQuerySink(server.Client(), server.URL+"/insertAll")
	if err := sink.Write(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if len(received.Rows) != 1 || received.Rows[0].InsertID != record.ID || received.Rows[0].JSON.ID != record.ID {
		t.Fatalf("wrong BigQuery insert body: %+v", received)
	}
}

func TestBigQuerySinkRejectsRowErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte(`{"insertErrors":[{"index":0,"errors":[{"reason":"invalid"}]}]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	sink := NewBigQuerySink(server.Client(), server.URL)
	now := time.Now().UTC()
	err := sink.Write(context.Background(), Record{
		ID: "fact-1", Kind: DataQualityFact,
		Provenance: Provenance{Class: "SIMULATED", SourceID: "source", SourceURI: "scenario:test", ObservedAt: now, IngestedAt: now, SchemaVersion: "1"},
		Payload:    json.RawMessage(`{"quality":"missing"}`),
	})
	if err == nil {
		t.Fatal("partial BigQuery insert was accepted")
	}
}

func TestECSTaskCredentialsConfigure(t *testing.T) {
	t.Setenv("GRIDOS_ANALYTICS", "local")
	if _, err := NewSink(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRIDOS_ANALYTICS", "bigquery")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/gridos-credentials.json")
	t.Setenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "/v2/credentials/task")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("GRIDOS_WIF_AUDIENCE", "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/gridos-aws/providers/ecs-task")
	t.Setenv("GRIDOS_WIF_SERVICE_ACCOUNT", "telemetry@example.iam.gserviceaccount.com")
	t.Setenv("GRIDOS_BIGQUERY_PROJECT", "example")
	t.Setenv("GRIDOS_BIGQUERY_DATASET", "telemetry")
	t.Setenv("GRIDOS_BIGQUERY_TABLE", "observations")
	if _, err := NewSink(context.Background()); err != nil {
		t.Fatal(err)
	}
}
