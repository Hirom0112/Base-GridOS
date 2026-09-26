package storage

import (
	"context"
	"fmt"
	"slices"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type TelemetryStore struct {
	pool *pgxpool.Pool
}

func NewTelemetryStore(pool *pgxpool.Pool) *TelemetryStore {
	return &TelemetryStore{pool: pool}
}

func (store *TelemetryStore) Write(ctx context.Context, observations []*gridosv1.TelemetryObservation) ([]*gridosv1.TelemetryObservation, error) {
	type row struct {
		observation *gridosv1.TelemetryObservation
		key         string
		values      []byte
	}
	rows := make([]row, 0, len(observations))
	keys := make([]string, 0, len(observations))
	seen := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		key := fmt.Sprintf("%s:%d", observation.GetDeviceId(), observation.GetSequence())
		if _, exists := seen[key]; exists {
			continue
		}
		values, err := protojson.Marshal(observation)
		if err != nil {
			return nil, err
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
		rows = append(rows, row{observation: observation, key: key, values: values})
	}
	if len(rows) == 0 {
		return nil, nil
	}
	slices.Sort(keys)
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended('telemetry:' || key, 0))
		FROM (SELECT unnest($1::text[]) AS key ORDER BY key) AS ordered`, keys); err != nil {
		return nil, err
	}
	existing := make(map[string]struct{})
	result, err := tx.Query(ctx, `
		SELECT resource_id
		FROM audit_journal
		WHERE action = 'TELEMETRY_RECEIVED' AND resource_id = ANY($1::text[])`, keys)
	if err != nil {
		return nil, err
	}
	for result.Next() {
		var key string
		if err = result.Scan(&key); err != nil {
			result.Close()
			return nil, err
		}
		existing[key] = struct{}{}
	}
	result.Close()
	if err = result.Err(); err != nil {
		return nil, err
	}
	inserted := make([]*gridosv1.TelemetryObservation, 0, len(rows))
	copyRows := make([][]any, 0, len(rows))
	for _, row := range rows {
		if _, exists := existing[row.key]; exists {
			continue
		}
		observation := row.observation
		copyRows = append(copyRows, []any{
			observation.GetObservationTime().AsTime(), observation.GetDeviceId(), "TELEMETRY_RECEIVED", "dispatch_event",
			row.key, nil, row.values, observation.GetObservationId(),
		})
		inserted = append(inserted, observation)
	}
	if len(copyRows) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"audit_journal"}, []string{
			"occurred_at", "actor_id", "action", "resource_type", "resource_id", "previous_values", "new_values", "correlation_id",
		}, pgx.CopyFromRows(copyRows))
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inserted, nil
}
