package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	storagegen "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIllegalTransition = errors.New("illegal event transition")

var eventTransitions = map[string]string{
	"REQUESTED":                 "PLANNED",
	"PLANNED":                   "VALIDATED",
	"VALIDATED":                 "APPROVED",
	"APPROVED":                  "COMMANDS_PERSISTED",
	"COMMANDS_PERSISTED":        "SENT",
	"SENT":                      "ACKNOWLEDGED_OR_UNCERTAIN",
	"ACKNOWLEDGED_OR_UNCERTAIN": "EXECUTING",
	"EXECUTING":                 "VERIFIED",
	"VERIFIED":                  "RECONCILED",
	"RECONCILED":                "REPORTED",
}

type EventTransition struct {
	EventID       string
	ExpectedState string
	NextState     string
	ActorID       string
	CorrelationID string
	OccurredAt    time.Time
}

type eventStateValue struct {
	State string `json:"state"`
}

func TransitionEvent(ctx context.Context, pool *pgxpool.Pool, transition EventTransition) (bool, error) {
	if eventTransitions[transition.ExpectedState] != transition.NextState {
		return false, ErrIllegalTransition
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storagegen.New(tx)
	_, err = queries.TransitionEventState(ctx, storagegen.TransitionEventStateParams{
		NextState:      transition.NextState,
		TransitionedAt: pgtype.Timestamptz{Time: transition.OccurredAt, Valid: true},
		EventID:        transition.EventID,
		ExpectedState:  transition.ExpectedState,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	previousValues, err := json.Marshal(eventStateValue{State: transition.ExpectedState})
	if err != nil {
		return false, err
	}
	newValues, err := json.Marshal(eventStateValue{State: transition.NextState})
	if err != nil {
		return false, err
	}
	_, err = queries.AppendAudit(ctx, storagegen.AppendAuditParams{
		OccurredAt:     pgtype.Timestamptz{Time: transition.OccurredAt, Valid: true},
		ActorID:        transition.ActorID,
		Action:         "EVENT_STATE_TRANSITIONED",
		ResourceType:   "dispatch_event",
		ResourceID:     transition.EventID,
		PreviousValues: previousValues,
		NewValues:      newValues,
		CorrelationID:  transition.CorrelationID,
	})
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
