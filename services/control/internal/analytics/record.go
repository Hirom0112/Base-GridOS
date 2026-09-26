package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Kind string

const (
	RawTelemetry        Kind = "raw_telemetry"
	NormalizedTelemetry Kind = "normalized_telemetry"
	ForecastAndActual   Kind = "forecast_and_actual"
	DispatchFact        Kind = "dispatch_fact"
	DataQualityFact     Kind = "data_quality_fact"
)

type Provenance struct {
	Class          string    `json:"class"`
	SourceID       string    `json:"source_id"`
	SourceURI      string    `json:"source_uri"`
	ObservedAt     time.Time `json:"observed_at"`
	IngestedAt     time.Time `json:"ingested_at"`
	SchemaVersion  string    `json:"schema_version"`
	SimulationSeed *int64    `json:"simulation_seed,omitempty"`
}

type Record struct {
	ID         string          `json:"id"`
	Kind       Kind            `json:"kind"`
	Provenance Provenance      `json:"provenance"`
	Payload    json.RawMessage `json:"payload"`
}

type Sink interface {
	Write(context.Context, Record) error
}

func (record Record) validate() error {
	if record.ID == "" {
		return errors.New("record id is required")
	}
	switch record.Kind {
	case RawTelemetry, NormalizedTelemetry, ForecastAndActual, DispatchFact, DataQualityFact:
	default:
		return errors.New("unknown analytics record kind")
	}
	switch record.Provenance.Class {
	case "CONFIRMED_PUBLIC", "CONFIRMED_SANDBOX", "AUTHORIZED_OPERATIONAL", "DERIVED", "SIMULATED":
	default:
		return errors.New("unknown provenance class")
	}
	if record.Provenance.SourceID == "" || record.Provenance.SourceURI == "" || record.Provenance.SchemaVersion == "" {
		return errors.New("source id, uri, and schema version are required")
	}
	if record.Provenance.ObservedAt.IsZero() || record.Provenance.IngestedAt.IsZero() {
		return errors.New("observation and ingestion times are required")
	}
	if !json.Valid(record.Payload) {
		return errors.New("payload must be valid JSON")
	}
	return nil
}
