package storage

import (
	"context"
	"errors"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	storagegen "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAcknowledgementDeadlinePending = errors.New("acknowledgement deadline has not passed")
var ErrInvalidReceiptStatus = errors.New("invalid receipt status")

type Acknowledgement struct {
	AcknowledgementID string
	CommandID         string
	IdempotencyKey    string
	ReceiptStatus     string
	ReceivedAt        time.Time
	GatewayID         string
	RejectionReason   string
	CorrelationID     string
}

type FeasiblePowerInterval struct {
	DeviceID                    string
	IntervalBegin               time.Time
	IntervalEnd                 time.Time
	LowerKW                     float64
	UpperKW                     float64
	LastConfirmedCommandID      string
	LastConfirmedSetpointKW     float64
	PossiblyAcceptedCommandID   string
	PossiblyAcceptedSetpointKW  float64
	PossiblyAcceptedEffectiveAt time.Time
	PossiblyAcceptedExpiresAt   time.Time
	MaxRampKWPerSecond          float64
	FreshTelemetryPowerKW       float64
	FreshTelemetryObservedAt    time.Time
	DerivedAt                   time.Time
	CorrelationID               string
}

func RecordAcknowledgement(ctx context.Context, pool *pgxpool.Pool, acknowledgement Acknowledgement) error {
	nextState := "ACKNOWLEDGED"
	if acknowledgement.ReceiptStatus == "REJECTED" {
		nextState = "REJECTED"
	} else if acknowledgement.ReceiptStatus != "ACCEPTED" {
		return ErrInvalidReceiptStatus
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockCommand(ctx, tx, acknowledgement.CommandID); err != nil {
		return err
	}
	var currentState string
	err = tx.QueryRow(ctx, `SELECT state FROM command_states
		WHERE command_id = $1 ORDER BY recorded_at DESC LIMIT 1`, acknowledgement.CommandID).Scan(&currentState)
	if err != nil {
		return err
	}
	if acknowledgement.ReceiptStatus == "ACCEPTED" && currentState != "SENT" && currentState != "UNCERTAIN" {
		return ErrIllegalCommandTransition
	}
	if acknowledgement.ReceiptStatus == "REJECTED" {
		if _, nonterminal := commandTransitions[currentState]; !nonterminal {
			return ErrIllegalCommandTransition
		}
	}
	queries := storagegen.New(tx)
	_, err = queries.UpsertCommandAcknowledgement(ctx, storagegen.UpsertCommandAcknowledgementParams{
		AcknowledgementID: acknowledgement.AcknowledgementID,
		CommandID:         acknowledgement.CommandID,
		IdempotencyKey:    acknowledgement.IdempotencyKey,
		ReceiptStatus:     acknowledgement.ReceiptStatus,
		ReceivedAt:        timestamp(acknowledgement.ReceivedAt),
		GatewayID:         acknowledgement.GatewayID,
		RejectionReason:   acknowledgement.RejectionReason,
		CorrelationID:     acknowledgement.CorrelationID,
	})
	if err != nil {
		return err
	}
	changed, err := appendCommandTransition(ctx, tx, acknowledgement.CommandID, []string{currentState}, nextState, time.Now().UTC(), acknowledgement.CorrelationID)
	if err != nil {
		return err
	}
	if !changed {
		return ErrIllegalCommandTransition
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	_ = observability.ProcessMetrics.RecordCommand(nextState)
	return nil
}

func MarkAcknowledgementUncertain(ctx context.Context, pool *pgxpool.Pool, deadline, now time.Time, interval FeasiblePowerInterval) (bool, error) {
	if now.Before(deadline) {
		return false, ErrAcknowledgementDeadlinePending
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockCommand(ctx, tx, interval.PossiblyAcceptedCommandID); err != nil {
		return false, err
	}
	var acknowledged bool
	err = tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM command_acknowledgements WHERE command_id = $1)", interval.PossiblyAcceptedCommandID).Scan(&acknowledged)
	if err != nil || acknowledged {
		return false, err
	}
	changed, err := appendCommandTransition(ctx, tx, interval.PossiblyAcceptedCommandID, []string{"SENT"}, "UNCERTAIN", now, interval.CorrelationID)
	if err != nil || !changed {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO uncertainty_intervals (
        device_id, interval_begin_time, interval_end_time,
        signed_feasible_power_lower_kw, signed_feasible_power_upper_kw,
        last_confirmed_command_id, last_confirmed_setpoint_kw,
        possibly_accepted_command_id, possibly_accepted_setpoint_kw,
        possibly_accepted_effective_at, possibly_accepted_expires_at,
        max_ramp_kw_per_second, fresh_telemetry_power_kw,
        fresh_telemetry_observed_at, derived_at, correlation_id
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		interval.DeviceID, interval.IntervalBegin, interval.IntervalEnd,
		interval.LowerKW, interval.UpperKW,
		interval.LastConfirmedCommandID, interval.LastConfirmedSetpointKW,
		interval.PossiblyAcceptedCommandID, interval.PossiblyAcceptedSetpointKW,
		interval.PossiblyAcceptedEffectiveAt, interval.PossiblyAcceptedExpiresAt,
		interval.MaxRampKWPerSecond, interval.FreshTelemetryPowerKW,
		interval.FreshTelemetryObservedAt, interval.DerivedAt, interval.CorrelationID)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func lockCommand(ctx context.Context, tx pgx.Tx, commandID string) error {
	var locked string
	return tx.QueryRow(ctx, "SELECT command_id FROM command_intents WHERE command_id = $1 FOR UPDATE", commandID).Scan(&locked)
}

func appendCommandTransition(ctx context.Context, tx pgx.Tx, commandID string, expected []string, next string, at time.Time, correlationID string) (bool, error) {
	tag, err := tx.Exec(ctx, `INSERT INTO command_states (command_id, state, recorded_at, correlation_id)
		SELECT $1, $3, GREATEST($4, latest.recorded_at + interval '1 microsecond'), $5
		FROM LATERAL (SELECT state, recorded_at FROM command_states WHERE command_id = $1 ORDER BY recorded_at DESC LIMIT 1) AS latest
		WHERE latest.state = ANY($2)`,
		commandID, expected, next, at, correlationID)
	return tag.RowsAffected() == 1, err
}
