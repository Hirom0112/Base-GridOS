package api

import (
	"context"
	"errors"
	"sync"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrNotFound       = errors.New("event not found")
	ErrInvalidState   = errors.New("event is not ready for this transition")
	ErrPlanVersion    = errors.New("plan version does not match")
	ErrIdempotencyKey = errors.New("idempotency key is required")
)

type AuditEntry struct {
	EventID    string
	Action     string
	Actor      string
	OccurredAt time.Time
}

type EventStore interface {
	Create(context.Context, *gridosv1.EventRequest, string, time.Time) (*gridosv1.DispatchEvent, error)
	Get(context.Context, string) (*gridosv1.DispatchEvent, map[gridosv1.ExclusionReason]uint64, error)
	Approve(context.Context, *gridosv1.ApproveEventRequest) (*gridosv1.DispatchEvent, error)
	Launch(context.Context, *gridosv1.LaunchEventRequest) (*gridosv1.DispatchEvent, error)
}

type MemoryEventStore struct {
	mu          sync.Mutex
	events      map[string]*gridosv1.DispatchEvent
	exclusions  map[string]map[gridosv1.ExclusionReason]uint64
	idempotency map[string]*gridosv1.DispatchEvent
	audit       []AuditEntry
}

func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		events:      make(map[string]*gridosv1.DispatchEvent),
		exclusions:  make(map[string]map[gridosv1.ExclusionReason]uint64),
		idempotency: make(map[string]*gridosv1.DispatchEvent),
	}
}

func (store *MemoryEventStore) Put(event *gridosv1.DispatchEvent) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.events[event.GetEventId()] = cloneEvent(event)
}

func (store *MemoryEventStore) SetExclusions(eventID string, exclusions map[gridosv1.ExclusionReason]uint64) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.exclusions[eventID] = cloneExclusions(exclusions)
}

func (store *MemoryEventStore) Audit() []AuditEntry {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]AuditEntry(nil), store.audit...)
}

func (store *MemoryEventStore) Create(_ context.Context, request *gridosv1.EventRequest, key string, now time.Time) (*gridosv1.DispatchEvent, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if key == "" {
		return nil, ErrIdempotencyKey
	}
	if prior := store.idempotency["create:"+key]; prior != nil {
		return cloneEvent(prior), nil
	}
	event := &gridosv1.DispatchEvent{
		EventId:       request.GetRequestId(),
		RequestId:     request.GetRequestId(),
		State:         gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED,
		CreatedAt:     timestamp(now),
		UpdatedAt:     timestamp(now),
		CorrelationId: request.GetCorrelationId(),
		Provenance:    simulatedProvenance(now),
	}
	store.events[event.GetEventId()] = event
	store.idempotency["create:"+key] = event
	return cloneEvent(event), nil
}

func (store *MemoryEventStore) Get(_ context.Context, eventID string) (*gridosv1.DispatchEvent, map[gridosv1.ExclusionReason]uint64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	event := store.events[eventID]
	if event == nil {
		return nil, nil, ErrNotFound
	}
	return cloneEvent(event), cloneExclusions(store.exclusions[eventID]), nil
}

func (store *MemoryEventStore) Approve(_ context.Context, request *gridosv1.ApproveEventRequest) (*gridosv1.DispatchEvent, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if request.GetIdempotencyKey() == "" {
		return nil, ErrIdempotencyKey
	}
	if prior := store.idempotency["approve:"+request.GetIdempotencyKey()]; prior != nil {
		return cloneEvent(prior), nil
	}
	event, err := store.transitionTarget(request.GetEventId(), request.GetPlanVersion(), gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED)
	if err != nil {
		return nil, err
	}
	event.State = gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED
	event.UpdatedAt = request.GetApprovedAt()
	store.audit = append(store.audit, AuditEntry{EventID: event.GetEventId(), Action: "EVENT_APPROVED", Actor: request.GetApprovedBy(), OccurredAt: request.GetApprovedAt().AsTime()})
	store.idempotency["approve:"+request.GetIdempotencyKey()] = cloneEvent(event)
	return cloneEvent(event), nil
}

func (store *MemoryEventStore) Launch(_ context.Context, request *gridosv1.LaunchEventRequest) (*gridosv1.DispatchEvent, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if request.GetIdempotencyKey() == "" {
		return nil, ErrIdempotencyKey
	}
	if prior := store.idempotency["launch:"+request.GetIdempotencyKey()]; prior != nil {
		return cloneEvent(prior), nil
	}
	event, err := store.transitionTarget(request.GetEventId(), request.GetPlanVersion(), gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED)
	if err != nil {
		return nil, err
	}
	event.State = gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_COMMANDS_PERSISTED
	event.UpdatedAt = request.GetRequestedAt()
	event.Launch = &gridosv1.EventLaunch{RequestedBy: request.GetRequestedBy(), RequestedAt: request.GetRequestedAt(), PlanVersion: request.GetPlanVersion()}
	store.audit = append(store.audit, AuditEntry{EventID: event.GetEventId(), Action: "EVENT_LAUNCHED", Actor: request.GetRequestedBy(), OccurredAt: request.GetRequestedAt().AsTime()})
	store.idempotency["launch:"+request.GetIdempotencyKey()] = cloneEvent(event)
	return cloneEvent(event), nil
}

func (store *MemoryEventStore) transitionTarget(eventID string, planVersion uint64, expected gridosv1.DispatchEventState) (*gridosv1.DispatchEvent, error) {
	event := store.events[eventID]
	if event == nil {
		return nil, ErrNotFound
	}
	if event.GetState() != expected {
		return nil, ErrInvalidState
	}
	if event.GetPlanVersion() != planVersion {
		return nil, ErrPlanVersion
	}
	return event, nil
}

func cloneEvent(event *gridosv1.DispatchEvent) *gridosv1.DispatchEvent {
	return proto.Clone(event).(*gridosv1.DispatchEvent)
}

func cloneExclusions(source map[gridosv1.ExclusionReason]uint64) map[gridosv1.ExclusionReason]uint64 {
	result := make(map[gridosv1.ExclusionReason]uint64, len(source))
	for reason, count := range source {
		result[reason] = count
	}
	return result
}

func timestamp(value time.Time) *timestamppb.Timestamp {
	return timestamppb.New(value)
}

func simulatedProvenance(at time.Time) *gridosv1.Provenance {
	return &gridosv1.Provenance{
		Provenance:    gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED,
		SourceId:      "fleet-file",
		ObservedAt:    timestamp(at),
		IngestedAt:    timestamp(at),
		SchemaVersion: "1",
	}
}
