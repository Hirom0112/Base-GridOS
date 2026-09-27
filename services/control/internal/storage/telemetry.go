package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

var ErrTelemetryExpired = errors.New("observation outside telemetry retention")

type TelemetryStore struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

type telemetryRow struct {
	observation *gridosv1.TelemetryObservation
	key         string
	values      []byte
}

func NewTelemetryStore(pool *pgxpool.Pool) *TelemetryStore {
	return NewTelemetryStoreAt(pool, time.Now)
}

func NewTelemetryStoreAt(pool *pgxpool.Pool, now func() time.Time) *TelemetryStore {
	return &TelemetryStore{pool: pool, now: now}
}

func (store *TelemetryStore) Write(ctx context.Context, gatewayID string, observations []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error) {
	if gatewayID == "" {
		return nil, errors.New("gateway identifier required")
	}
	rows, keys, err := telemetryRows(observations)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('telemetry:' || key, 0)) FROM (SELECT unnest($1::text[]) AS key ORDER BY key) AS ordered`, keys); err != nil {
		return nil, err
	}
	receivedAt := store.now()
	cutoff := retentionStart(receivedAt)
	expired, err := rejectExpiredRows(ctx, tx, rows, cutoff)
	if err != nil {
		return nil, err
	}
	if expired {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, ErrTelemetryExpired
	}
	if err = ensureTelemetryPartitions(ctx, tx, rows); err != nil {
		return nil, err
	}
	existing, err := existingTelemetry(ctx, tx, rows, cutoff)
	if err != nil {
		return nil, err
	}
	inserted := make([]*gridosv1.TelemetryObservation, 0, len(rows))
	copyRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		if _, exists := existing[row.key]; exists {
			continue
		}
		observation := row.observation
		copyRows = append(copyRows, []any{observation.GetObservationTime().AsTime(), observation.GetDeviceId(), int64(observation.GetSequence()), observation.GetObservationId(), row.values})
		inserted = append(inserted, observation)
	}
	if len(copyRows) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"telemetry_observations"}, []string{"observed_at", "device_id", "sequence", "observation_id", "payload"}, pgx.CopyFromRows(copyRows))
		if err != nil {
			return nil, err
		}
	}
	if err = recordGatewayPublication(ctx, tx, gatewayID, receivedAt, len(rows), inserted); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inserted, nil
}

func rejectExpiredRows(ctx context.Context, tx pgx.Tx, rows []telemetryRow, cutoff time.Time) (bool, error) {
	expired := false
	for _, row := range rows {
		if row.observation.GetObservationTime().AsTime().Before(cutoff) {
			if err := rejectExpired(ctx, tx, row); err != nil {
				return false, err
			}
			expired = true
		}
	}
	return expired, nil
}

func recordGatewayPublication(ctx context.Context, tx pgx.Tx, gatewayID string, receivedAt time.Time, count int, inserted []*gridosv1.TelemetryObservation) error {
	_, err := tx.Exec(ctx, `INSERT INTO gateway_heartbeats(gateway_id,last_published_at,last_sequence_count)
		VALUES ($1,$2,$3) ON CONFLICT (gateway_id) DO UPDATE
		SET last_published_at = GREATEST(gateway_heartbeats.last_published_at, EXCLUDED.last_published_at),
		last_sequence_count = CASE WHEN EXCLUDED.last_published_at >= gateway_heartbeats.last_published_at
		THEN EXCLUDED.last_sequence_count ELSE gateway_heartbeats.last_sequence_count END`, gatewayID, receivedAt, count)
	if err != nil {
		return err
	}
	latest := make(map[string]time.Time, len(inserted))
	for _, observation := range inserted {
		at := observation.GetObservationTime().AsTime()
		if at.After(latest[observation.GetDeviceId()]) {
			latest[observation.GetDeviceId()] = at
		}
	}
	if len(latest) > 0 {
		deviceIDs := make([]string, 0, len(latest))
		times := make([]time.Time, 0, len(latest))
		for deviceID, at := range latest {
			deviceIDs = append(deviceIDs, deviceID)
			times = append(times, at)
		}
		_, err = tx.Exec(ctx, `INSERT INTO gateway_device_sources(device_id,gateway_id,observed_at)
			SELECT device_id,$1,observed_at FROM unnest($2::text[],$3::timestamptz[]) AS source(device_id,observed_at)
			ON CONFLICT (device_id) DO UPDATE SET gateway_id = EXCLUDED.gateway_id, observed_at = EXCLUDED.observed_at
			WHERE EXCLUDED.observed_at > gateway_device_sources.observed_at`, gatewayID, deviceIDs, times)
		if err != nil {
			return err
		}
	}
	return nil
}

func telemetryRows(observations []*gridosv1.TelemetryObservation) ([]telemetryRow, []string, error) {
	rows := make([]telemetryRow, 0, len(observations))
	keys := make([]string, 0, len(observations))
	seen := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		if observation == nil || observation.GetObservationTime() == nil || !observation.GetObservationTime().IsValid() || observation.GetDeviceId() == "" || observation.GetSequence() == 0 || observation.GetSequence() > math.MaxInt64 {
			return nil, nil, errors.New("invalid telemetry observation")
		}
		key := fmt.Sprintf("%s:%d", observation.GetDeviceId(), observation.GetSequence())
		if _, exists := seen[key]; exists {
			continue
		}
		values, err := protojson.Marshal(observation)
		if err != nil {
			return nil, nil, err
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
		rows = append(rows, telemetryRow{observation: observation, key: key, values: values})
	}
	slices.Sort(keys)
	return rows, keys, nil
}

func rejectExpired(ctx context.Context, tx pgx.Tx, row telemetryRow) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		SELECT $1, 'TELEMETRY_REJECTED', 'telemetry_observation', $2, $3, $4
		WHERE NOT EXISTS (SELECT 1 FROM audit_journal WHERE action = 'TELEMETRY_REJECTED' AND resource_id = $2)`, row.observation.GetDeviceId(), row.key, row.values, row.observation.GetObservationId())
	return err
}

func existingTelemetry(ctx context.Context, tx pgx.Tx, rows []telemetryRow, cutoff time.Time) (map[string]struct{}, error) {
	devices := make([]string, 0, len(rows))
	sequences := make([]int64, 0, len(rows))
	for _, row := range rows {
		devices = append(devices, row.observation.GetDeviceId())
		sequences = append(sequences, int64(row.observation.GetSequence()))
	}
	result, err := tx.Query(ctx, `SELECT observation.device_id, observation.sequence
		FROM unnest($1::text[], $2::bigint[]) AS wanted(device_id, sequence)
		JOIN telemetry_observations AS observation USING (device_id, sequence)
		WHERE observation.observed_at >= $3`, devices, sequences, cutoff)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	existing := make(map[string]struct{})
	for result.Next() {
		var deviceID string
		var sequence int64
		if err = result.Scan(&deviceID, &sequence); err != nil {
			return nil, err
		}
		existing[fmt.Sprintf("%s:%d", deviceID, sequence)] = struct{}{}
	}
	return existing, result.Err()
}
