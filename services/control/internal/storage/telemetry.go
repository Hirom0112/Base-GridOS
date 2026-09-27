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
}

type telemetryRow struct {
	observation *gridosv1.TelemetryObservation
	key         string
	values      []byte
}

func NewTelemetryStore(pool *pgxpool.Pool) *TelemetryStore {
	return &TelemetryStore{pool: pool}
}

func (store *TelemetryStore) Write(ctx context.Context, observations []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error) {
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
	cutoff := retentionStart(time.Now().UTC())
	expired := false
	for _, row := range rows {
		if row.observation.GetObservationTime().AsTime().Before(cutoff) {
			if err = rejectExpired(ctx, tx, row); err != nil {
				return nil, err
			}
			expired = true
		}
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
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inserted, nil
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
