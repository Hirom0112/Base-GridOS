package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	storagegen "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/gen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CommandIntent struct {
	CommandID      string
	IdempotencyKey string
	DeviceID       string
	EventID        string
	PlanVersion    int64
	Generation     int64
	SetpointKW     float64
	IssuedAt       time.Time
	EffectiveAt    time.Time
	ExpiresAt      time.Time
	PolicyVersion  string
	CorrelationID  string
}

type OutboxClaim struct {
	AvailableAt time.Time
	LeaseUntil  time.Time
	BatchSize   int
}

type ClaimedCommand struct {
	CommandIntent
	Attempts int
}

type OutboxPublisher interface {
	Publish(context.Context, ClaimedCommand) error
}

func InsertCommand(ctx context.Context, pool *pgxpool.Pool, command CommandIntent) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storagegen.New(tx)
	_, err = queries.InsertCommandIntentAndOutbox(ctx, storagegen.InsertCommandIntentAndOutboxParams{
		CommandID:      command.CommandID,
		IdempotencyKey: command.IdempotencyKey,
		DeviceID:       command.DeviceID,
		EventID:        command.EventID,
		PlanVersion:    command.PlanVersion,
		Generation:     command.Generation,
		SetpointKw:     command.SetpointKW,
		IssuedAt:       timestamp(command.IssuedAt),
		EffectiveAt:    timestamp(command.EffectiveAt),
		ExpiresAt:      timestamp(command.ExpiresAt),
		PolicyVersion:  command.PolicyVersion,
		CorrelationID:  command.CorrelationID,
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO command_states
        (command_id, state, recorded_at, correlation_id)
		VALUES ($1, 'PERSISTED', clock_timestamp(), $2)`, command.CommandID, command.CorrelationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func InsertZeroCommand(ctx context.Context, pool *pgxpool.Pool, command CommandIntent) error {
	if command.SetpointKW != 0 || command.IdempotencyKey == "" {
		return errors.New("idempotent zero command required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `INSERT INTO command_intents
		(command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9, $10, $11)
		ON CONFLICT (idempotency_key) DO NOTHING`, command.CommandID, command.IdempotencyKey, command.DeviceID, command.EventID,
		command.PlanVersion, command.Generation, command.IssuedAt, command.EffectiveAt, command.ExpiresAt, command.PolicyVersion, command.CorrelationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var existing CommandIntent
		err = tx.QueryRow(ctx, `SELECT command_id, device_id, event_id, plan_version, generation, setpoint_kw
			FROM command_intents WHERE idempotency_key = $1`, command.IdempotencyKey).Scan(
			&existing.CommandID, &existing.DeviceID, &existing.EventID, &existing.PlanVersion, &existing.Generation, &existing.SetpointKW)
		if err != nil {
			return err
		}
		if existing.CommandID != command.CommandID || existing.DeviceID != command.DeviceID || existing.EventID != command.EventID || existing.PlanVersion != command.PlanVersion || existing.Generation != command.Generation || existing.SetpointKW != 0 {
			return fmt.Errorf("conflicting reuse of zero command key %s", command.IdempotencyKey)
		}
		return nil
	}
	if _, err = tx.Exec(ctx, `INSERT INTO command_outbox (command_id, state, correlation_id) VALUES ($1, 'PENDING', $2)`, command.CommandID, command.CorrelationID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO command_states (command_id, state, recorded_at, correlation_id) VALUES ($1, 'PERSISTED', clock_timestamp(), $2)`, command.CommandID, command.CorrelationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ClaimOutbox(ctx context.Context, pool *pgxpool.Pool, claim OutboxClaim) ([]ClaimedCommand, error) {
	rows, err := pool.Query(ctx, `WITH candidates AS (
        SELECT command_id
        FROM command_outbox
        WHERE state IN ('PENDING', 'PUBLISHING') AND next_attempt_at <= $1
        ORDER BY next_attempt_at, command_id
        FOR UPDATE SKIP LOCKED
        LIMIT $2
    ), claimed AS (
        UPDATE command_outbox AS outbox
        SET state = 'PUBLISHING', attempts = outbox.attempts + 1, next_attempt_at = $3
        FROM candidates
        WHERE outbox.command_id = candidates.command_id
        RETURNING outbox.command_id, outbox.attempts
    )
    SELECT intent.command_id, intent.idempotency_key, intent.device_id,
           intent.event_id, intent.plan_version, intent.generation,
           intent.setpoint_kw, intent.issued_at, intent.effective_at,
           intent.expires_at, intent.policy_version, intent.correlation_id,
           claimed.attempts
    FROM claimed JOIN command_intents AS intent USING (command_id)
    ORDER BY intent.command_id`, claim.AvailableAt, claim.BatchSize, claim.LeaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	commands := make([]ClaimedCommand, 0)
	for rows.Next() {
		var command ClaimedCommand
		err = rows.Scan(
			&command.CommandID,
			&command.IdempotencyKey,
			&command.DeviceID,
			&command.EventID,
			&command.PlanVersion,
			&command.Generation,
			&command.SetpointKW,
			&command.IssuedAt,
			&command.EffectiveAt,
			&command.ExpiresAt,
			&command.PolicyVersion,
			&command.CorrelationID,
			&command.Attempts,
		)
		if err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	return commands, rows.Err()
}

func MarkOutboxPublished(ctx context.Context, pool *pgxpool.Pool, commandID string, publishedAt time.Time) error {
	tag, err := pool.Exec(ctx, `UPDATE command_outbox
		SET state = 'PUBLISHED', published_at = $2
		WHERE command_id = $1 AND state = 'PUBLISHING'`, commandID, publishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("outbox row is not publishing")
	}
	return nil
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
