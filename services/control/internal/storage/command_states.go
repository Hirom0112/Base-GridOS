package storage

import (
	"context"
	"errors"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIllegalCommandTransition = errors.New("illegal command transition")

var commandTransitions = map[string]map[string]observability.CommandState{
	"PERSISTED": {
		"SENT": observability.CommandSent, "EXPIRED": observability.CommandExpired, "REJECTED": observability.CommandRejected,
	},
	"SENT": {
		"ACKNOWLEDGED": observability.CommandAcknowledged, "UNCERTAIN": observability.CommandUncertain,
		"EXPIRED": observability.CommandExpired, "REJECTED": observability.CommandRejected,
	},
	"ACKNOWLEDGED": {
		"EXECUTING": observability.CommandExecuting, "CANCELLED": observability.CommandCancelled,
		"EXPIRED": observability.CommandExpired, "REJECTED": observability.CommandRejected,
	},
	"UNCERTAIN": {
		"ACKNOWLEDGED": observability.CommandAcknowledged, "EXECUTING": observability.CommandExecuting,
		"REJECTED": observability.CommandRejected, "CANCELLED": observability.CommandCancelled, "EXPIRED": observability.CommandExpired,
	},
	"EXECUTING": {
		"COMPLETED": observability.CommandCompleted, "CANCELLED": observability.CommandCancelled,
		"EXPIRED": observability.CommandExpired, "REJECTED": observability.CommandRejected,
	},
}

type CommandTransition struct {
	CommandID     string
	ExpectedState string
	NextState     string
	OccurredAt    time.Time
	CorrelationID string
}

func TransitionCommand(ctx context.Context, pool *pgxpool.Pool, transition CommandTransition) (bool, error) {
	nextState, allowed := commandTransitions[transition.ExpectedState][transition.NextState]
	if !allowed {
		return false, ErrIllegalCommandTransition
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockCommand(ctx, tx, transition.CommandID); err != nil {
		return false, err
	}
	changed, err := appendCommandTransition(ctx, tx, transition.CommandID, []string{transition.ExpectedState}, transition.NextState, transition.OccurredAt, transition.CorrelationID)
	if err != nil || !changed {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	observability.ProcessMetrics.RecordCommand(nextState)
	return true, nil
}

func UncertainCommandCount(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var count int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM (
		SELECT DISTINCT ON (command_id) state
		FROM command_states
		ORDER BY command_id, recorded_at DESC
	) AS latest WHERE state = 'UNCERTAIN'`).Scan(&count)
	return count, err
}
