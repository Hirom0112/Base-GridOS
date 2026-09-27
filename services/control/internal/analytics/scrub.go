package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type ScrubbedSink struct {
	next Sink
	key  []byte
}

func NewScrubbedSink(next Sink, key []byte) (*ScrubbedSink, error) {
	if next == nil || len(key) < 16 {
		return nil, errors.New("analytics scrubber requires sink and key")
	}
	return &ScrubbedSink{next: next, key: append([]byte(nil), key...)}, nil
}

func (s *ScrubbedSink) Write(ctx context.Context, record Record) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(record.Payload, &fields); err != nil {
		return err
	}
	clean := make(map[string]json.RawMessage)
	for key, value := range fields {
		switch key {
		case "count", "duration_ms", "power_kw", "energy_kwh":
			var number json.Number
			if err := json.Unmarshal(value, &number); err == nil {
				clean[key] = value
			}
		}
	}
	payload, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	record.ID = s.digest(record.ID)
	record.Provenance.SourceID = s.digest(record.Provenance.SourceID)
	record.Provenance.SourceURI = "redacted"
	record.Payload = payload
	return s.next.Write(ctx, record)
}

func (s *ScrubbedSink) digest(value string) string {
	hash := hmac.New(sha256.New, s.key)
	_, _ = hash.Write([]byte(value))
	return hex.EncodeToString(hash.Sum(nil))
}
