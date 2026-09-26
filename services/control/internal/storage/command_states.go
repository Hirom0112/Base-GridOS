package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIllegalCommandTransition = errors.New("illegal command transition")

var commandTransitions = map[string]map[string]struct{}{
	"PERSISTED": {
		"SENT": {}, "EXPIRED": {}, "REJECTED": {},
	},
	"SENT": {
		"ACKNOWLEDGED": {}, "UNCERTAIN": {}, "EXPIRED": {}, "REJECTED": {},
	},
	"ACKNOWLEDGED": {
		"EXECUTING": {}, "CANCELLED": {}, "EXPIRED": {}, "REJECTED": {},
	},
	"UNCERTAIN": {
		"EXECUTING": {}, "REJECTED": {}, "CANCELLED": {}, "EXPIRED": {},
	},
	"EXECUTING": {
		"COMPLETED": {}, "CANCELLED": {}, "EXPIRED": {}, "REJECTED": {},
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
	if _, allowed := commandTransitions[transition.ExpectedState][transition.NextState]; !allowed {
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
	return true, nil
}
