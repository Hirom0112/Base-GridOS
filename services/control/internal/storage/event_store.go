package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	storagegen "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrEventNotFound       = errors.New("event not found")
	ErrEventState          = errors.New("event is not ready for this transition")
	ErrEventPlanVersion    = errors.New("event plan version does not match")
	ErrEventIdempotencyKey = errors.New("event idempotency key is required")
)

type PostgresEventStore struct {
	pool *pgxpool.Pool
}

type AuditRecord struct {
	OccurredAt     time.Time
	ActorID        string
	Action         string
	ResourceID     string
	PreviousValues []byte
	NewValues      []byte
	CorrelationID  string
}

type launchValues struct {
	RequestedBy    string    `json:"requested_by"`
	RequestedAt    time.Time `json:"requested_at"`
	PlanVersion    uint64    `json:"plan_version"`
	IdempotencyKey string    `json:"idempotency_key"`
}

type idempotencyValues struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type storedExclusion struct {
	Reason json.RawMessage `json:"reason"`
}

func NewPostgresEventStore(pool *pgxpool.Pool) *PostgresEventStore {
	return &PostgresEventStore{pool: pool}
}

func AppendAudit(ctx context.Context, db storagegen.DBTX, record AuditRecord) error {
	_, err := storagegen.New(db).AppendAudit(ctx, storagegen.AppendAuditParams{
		OccurredAt: timestamp(record.OccurredAt), ActorID: record.ActorID, Action: record.Action,
		ResourceType: "dispatch_event", ResourceID: record.ResourceID,
		PreviousValues: record.PreviousValues, NewValues: record.NewValues, CorrelationID: record.CorrelationID,
	})
	return err
}

func (store *PostgresEventStore) Create(ctx context.Context, request *gridosv1.EventRequest, key string, now time.Time) (*gridosv1.DispatchEvent, error) {
	if key == "" {
		return nil, ErrEventIdempotencyKey
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockIdempotency(ctx, tx, "create:"+key); err != nil {
		return nil, err
	}
	queries := storagegen.New(tx)
	prior, err := queries.FindAuditResourceByIdempotency(ctx, storagegen.FindAuditResourceByIdempotencyParams{Action: "EVENT_REQUEST_CREATED", IdempotencyKey: key})
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return store.event(ctx, prior)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if request == nil || request.GetRequestId() == "" || request.GetBeginTime() == nil || request.GetEndTime() == nil {
		return nil, errors.New("complete event request required")
	}
	if err = queries.InsertDispatchRequest(ctx, storagegen.InsertDispatchRequestParams{
		RequestID: request.GetRequestId(), EventType: request.GetEventType(), BeginTime: timestamp(request.GetBeginTime().AsTime()),
		EndTime: timestamp(request.GetEndTime().AsTime()), TargetKw: request.GetTargetKw(), MeasurementBoundary: enumSuffix(request.GetMeasurementBoundary().String(), "MEASUREMENT_BOUNDARY_"),
		LoadZones: request.GetLoadZones(), CorrelationID: request.GetCorrelationId(), CreatedAt: timestamp(now),
	}); err != nil {
		return nil, err
	}
	row, err := queries.InsertDispatchEvent(ctx, storagegen.InsertDispatchEventParams{
		EventID: request.GetRequestId(), RequestID: request.GetRequestId(), State: "REQUESTED", PlanVersion: 0,
		CorrelationID: request.GetCorrelationId(), CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
	})
	if err != nil {
		return nil, err
	}
	values, err := json.Marshal(idempotencyValues{IdempotencyKey: key})
	if err != nil {
		return nil, err
	}
	if err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: now, ActorID: "service", Action: "EVENT_REQUEST_CREATED", ResourceID: row.EventID, NewValues: values, CorrelationID: row.CorrelationID}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return eventFromRow(row, nil), nil
}

func (store *PostgresEventStore) Get(ctx context.Context, eventID string) (*gridosv1.DispatchEvent, map[gridosv1.ExclusionReason]uint64, error) {
	event, err := store.event(ctx, eventID)
	if err != nil {
		return nil, nil, err
	}
	exclusions, err := store.exclusions(ctx, eventID)
	if err != nil {
		return nil, nil, err
	}
	return event, exclusions, nil
}

func (store *PostgresEventStore) Approve(ctx context.Context, request *gridosv1.ApproveEventRequest) (*gridosv1.DispatchEvent, error) {
	if request.GetIdempotencyKey() == "" {
		return nil, ErrEventIdempotencyKey
	}
	return store.transition(ctx, transitionRequest{
		EventID: request.GetEventId(), PlanVersion: request.GetPlanVersion(), IdempotencyKey: request.GetIdempotencyKey(),
		Actor: request.GetApprovedBy(), At: request.GetApprovedAt().AsTime(), Expected: "VALIDATED", Next: "APPROVED", Action: "EVENT_APPROVED",
	})
}

func (store *PostgresEventStore) Launch(ctx context.Context, request *gridosv1.LaunchEventRequest) (*gridosv1.DispatchEvent, error) {
	if request.GetIdempotencyKey() == "" {
		return nil, ErrEventIdempotencyKey
	}
	return store.transition(ctx, transitionRequest{
		EventID: request.GetEventId(), PlanVersion: request.GetPlanVersion(), IdempotencyKey: request.GetIdempotencyKey(),
		Actor: request.GetRequestedBy(), At: request.GetRequestedAt().AsTime(), Expected: "APPROVED", Next: "COMMANDS_PERSISTED", Action: "EVENT_LAUNCHED",
	})
}

type transitionRequest struct {
	EventID        string
	PlanVersion    uint64
	IdempotencyKey string
	Actor          string
	At             time.Time
	Expected       string
	Next           string
	Action         string
}

func (store *PostgresEventStore) transition(ctx context.Context, request transitionRequest) (*gridosv1.DispatchEvent, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockIdempotency(ctx, tx, strings.ToLower(request.Action)+":"+request.IdempotencyKey); err != nil {
		return nil, err
	}
	queries := storagegen.New(tx)
	prior, err := queries.FindAuditResourceByIdempotency(ctx, storagegen.FindAuditResourceByIdempotencyParams{Action: request.Action, IdempotencyKey: request.IdempotencyKey})
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return store.event(ctx, prior)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	row, err := queries.LockControlEvent(ctx, request.EventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.State != request.Expected {
		return nil, ErrEventState
	}
	if row.PlanVersion != int64(request.PlanVersion) {
		return nil, ErrEventPlanVersion
	}
	if request.Action == "EVENT_APPROVED" {
		err = queries.InsertOperatorApproval(ctx, storagegen.InsertOperatorApprovalParams{
			ApprovalID: "approval-" + request.IdempotencyKey, EventID: request.EventID, PlanVersion: int64(request.PlanVersion),
			DecidedBy: request.Actor, DecidedAt: timestamp(request.At), CorrelationID: row.CorrelationID,
		})
		if err != nil {
			return nil, err
		}
	}
	updated, err := queries.TransitionEventState(ctx, storagegen.TransitionEventStateParams{NextState: request.Next, TransitionedAt: timestamp(request.At), EventID: request.EventID, ExpectedState: request.Expected})
	if err != nil {
		return nil, err
	}
	previous, next, err := transitionAuditValues(request)
	if err != nil {
		return nil, err
	}
	if err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: request.At, ActorID: request.Actor, Action: "EVENT_STATE_TRANSITIONED", ResourceID: request.EventID, PreviousValues: previous, NewValues: next, CorrelationID: row.CorrelationID}); err != nil {
		return nil, err
	}
	if err = AppendAudit(ctx, tx, AuditRecord{OccurredAt: request.At, ActorID: request.Actor, Action: request.Action, ResourceID: request.EventID, NewValues: next, CorrelationID: row.CorrelationID}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	var launch *gridosv1.EventLaunch
	if request.Action == "EVENT_LAUNCHED" {
		launch = &gridosv1.EventLaunch{RequestedBy: request.Actor, RequestedAt: timestamppb.New(request.At), PlanVersion: request.PlanVersion}
	}
	return eventFromRow(updated, launch), nil
}

func (store *PostgresEventStore) event(ctx context.Context, eventID string) (*gridosv1.DispatchEvent, error) {
	queries := storagegen.New(store.pool)
	row, err := queries.GetControlEvent(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, err
	}
	var launch *gridosv1.EventLaunch
	values, launchErr := queries.LatestEventLaunch(ctx, eventID)
	if launchErr == nil {
		var stored launchValues
		if err = json.Unmarshal(values, &stored); err != nil {
			return nil, err
		}
		launch = &gridosv1.EventLaunch{RequestedBy: stored.RequestedBy, RequestedAt: timestamppb.New(stored.RequestedAt), PlanVersion: stored.PlanVersion}
	} else if !errors.Is(launchErr, pgx.ErrNoRows) {
		return nil, launchErr
	}
	return eventFromRow(row, launch), nil
}

func (store *PostgresEventStore) exclusions(ctx context.Context, eventID string) (map[gridosv1.ExclusionReason]uint64, error) {
	contents, err := storagegen.New(store.pool).LatestEligibilityExclusions(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[gridosv1.ExclusionReason]uint64{}, nil
	}
	if err != nil {
		return nil, err
	}
	var stored []storedExclusion
	if err = json.Unmarshal(contents, &stored); err != nil {
		return nil, err
	}
	result := make(map[gridosv1.ExclusionReason]uint64)
	for _, exclusion := range stored {
		reason, parseErr := exclusionReason(exclusion.Reason)
		if parseErr != nil {
			return nil, parseErr
		}
		result[reason]++
	}
	return result, nil
}

func eventFromRow(row storagegen.DispatchEvent, launch *gridosv1.EventLaunch) *gridosv1.DispatchEvent {
	state := gridosv1.DispatchEventState(gridosv1.DispatchEventState_value["DISPATCH_EVENT_STATE_"+row.State])
	return &gridosv1.DispatchEvent{
		EventId: row.EventID, RequestId: row.RequestID, State: state, PlanVersion: uint64(row.PlanVersion),
		CreatedAt: timestamppb.New(row.CreatedAt.Time), UpdatedAt: timestamppb.New(row.UpdatedAt.Time),
		CorrelationId: row.CorrelationID, Launch: launch,
	}
}

func transitionAuditValues(request transitionRequest) ([]byte, []byte, error) {
	previous, err := json.Marshal(eventStateValue{State: request.Expected})
	if err != nil {
		return nil, nil, err
	}
	if request.Action == "EVENT_LAUNCHED" {
		next, marshalErr := json.Marshal(launchValues{RequestedBy: request.Actor, RequestedAt: request.At, PlanVersion: request.PlanVersion, IdempotencyKey: request.IdempotencyKey})
		return previous, next, marshalErr
	}
	next, err := json.Marshal(struct {
		State          string `json:"state"`
		IdempotencyKey string `json:"idempotency_key"`
	}{State: request.Next, IdempotencyKey: request.IdempotencyKey})
	return previous, next, err
}

func exclusionReason(raw json.RawMessage) (gridosv1.ExclusionReason, error) {
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		value, found := gridosv1.ExclusionReason_value[name]
		if !found {
			return 0, fmt.Errorf("unknown exclusion reason %q", name)
		}
		return gridosv1.ExclusionReason(value), nil
	}
	value, err := strconv.ParseInt(string(raw), 10, 32)
	return gridosv1.ExclusionReason(value), err
}

func lockIdempotency(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", key)
	return err
}

func enumSuffix(value, prefix string) string {
	return strings.TrimPrefix(value, prefix)
}
