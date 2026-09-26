package storage

import (
	"context"
	"fmt"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
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
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	inserted := make([]*gridosv1.TelemetryObservation, 0, len(observations))
	for _, observation := range observations {
		key := fmt.Sprintf("%s:%d", observation.GetDeviceId(), observation.GetSequence())
		if err = lockIdempotency(ctx, tx, "telemetry:"+key); err != nil {
			return nil, err
		}
		var exists bool
		err = tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM audit_journal WHERE action = 'TELEMETRY_RECEIVED' AND resource_id = $1)", key).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		values, marshalErr := protojson.Marshal(observation)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if err = AppendAudit(ctx, tx, AuditRecord{
			OccurredAt: observation.GetObservationTime().AsTime(), ActorID: observation.GetDeviceId(), Action: "TELEMETRY_RECEIVED",
			ResourceID: key, NewValues: values, CorrelationID: observation.GetObservationId(),
		}); err != nil {
			return nil, err
		}
		inserted = append(inserted, observation)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inserted, nil
}
