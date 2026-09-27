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
	pool  *pgxpool.Pool
	store *storage.PostgresEventStore
}

func NewPostgresEventStore(pool *pgxpool.Pool) *PostgresEventStore {
	return &PostgresEventStore{pool: pool, store: storage.NewPostgresEventStore(pool)}
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

func (store *PostgresEventStore) StorePlanned(ctx context.Context, eventID string, request *gridosv1.OptimizationRequest, plan *gridosv1.DispatchPlan, at time.Time) (*gridosv1.DispatchEvent, error) {
	event, err := store.store.StorePlanned(ctx, eventID, request, plan, at)
	return event, eventStoreError(err)
}

func (store *PostgresEventStore) ValidatePlanned(ctx context.Context, eventID string, planVersion uint64, violations []storage.StoredViolation, at time.Time) (*gridosv1.DispatchEvent, error) {
	event, err := store.store.ValidatePlanned(ctx, eventID, planVersion, violations, at)
	return event, eventStoreError(err)
}

func (store *PostgresEventStore) LoadPlan(ctx context.Context, eventID string, planVersion uint64) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return store.store.LoadPlan(ctx, eventID, planVersion)
}

func (store *PostgresEventStore) LoadPlanManifest(ctx context.Context, eventID string, planVersion uint64) (*gridosv1.PlanManifest, error) {
	return store.store.LoadPlanManifest(ctx, eventID, planVersion)
}

func (store *PostgresEventStore) Advance(ctx context.Context, eventID, expected, next, actor string, at time.Time) (*gridosv1.DispatchEvent, error) {
	event, err := store.store.Advance(ctx, eventID, expected, next, actor, at)
	return event, eventStoreError(err)
}

func (store *PostgresEventStore) Violations(ctx context.Context, eventID string) ([]storage.StoredViolation, error) {
	return store.store.Violations(ctx, eventID)
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
