package api

import (
	"context"
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresEventStore struct {
	store *storage.PostgresEventStore
}

func NewPostgresEventStore(pool *pgxpool.Pool) *PostgresEventStore {
	return &PostgresEventStore{store: storage.NewPostgresEventStore(pool)}
}

func (store *PostgresEventStore) Create(ctx context.Context, request *gridosv1.EventRequest, key string, now time.Time) (*gridosv1.DispatchEvent, error) {
	event, err := store.store.Create(ctx, request, key, now)
	return event, eventStoreError(err)
}

func (store *PostgresEventStore) Get(ctx context.Context, eventID string) (*gridosv1.DispatchEvent, map[gridosv1.ExclusionReason]uint64, error) {
	event, exclusions, err := store.store.Get(ctx, eventID)
	return event, exclusions, eventStoreError(err)
}

func (store *PostgresEventStore) Approve(ctx context.Context, request *gridosv1.ApproveEventRequest) (*gridosv1.DispatchEvent, error) {
	event, err := store.store.Approve(ctx, request)
	return event, eventStoreError(err)
}

func (store *PostgresEventStore) Launch(ctx context.Context, request *gridosv1.LaunchEventRequest) (*gridosv1.DispatchEvent, error) {
	event, err := store.store.Launch(ctx, request)
	return event, eventStoreError(err)
}

func eventStoreError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, storage.ErrEventNotFound):
		return ErrNotFound
	case errors.Is(err, storage.ErrEventState):
		return ErrInvalidState
	case errors.Is(err, storage.ErrEventPlanVersion):
		return ErrPlanVersion
	case errors.Is(err, storage.ErrEventIdempotencyKey):
		return ErrIdempotencyKey
	default:
		return err
	}
}
