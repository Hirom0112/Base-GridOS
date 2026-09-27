package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type AwayPeriod struct {
	ID               string
	MemberID         string
	Start            time.Time
	End              time.Time
	ConsentVersion   string
	CorrelationID    string
	EndedAt          *time.Time
	EndID            string
	EndCorrelationID string
}

type EndAway struct {
	ID            string
	PeriodID      string
	MemberID      string
	At            time.Time
	CorrelationID string
}

func (period AwayPeriod) validate() error {
	if period.ID == "" || period.MemberID == "" || period.ConsentVersion == "" || period.CorrelationID == "" || period.Start.IsZero() || !period.End.After(period.Start) {
		return errors.New("away period requires identity, consent version, and ordered times")
	}
	return nil
}

func (store *Store) ScheduleAway(ctx context.Context, period AwayPeriod) error {
	if err := period.validate(); err != nil {
		return err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "member-away:"+period.MemberID); err != nil {
		return err
	}
	previous, err := awayPeriodByID(ctx, tx, period.ID)
	if err != nil {
		return err
	}
	if previous != nil {
		if sameAwayPeriod(*previous, period) {
			return nil
		}
		return errors.New("away period idempotency key has different terms")
	}
	var overlaps bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM member_away_periods
		WHERE member_id = $1 AND start_time < $2 AND COALESCE(ended_at, end_time) > $3)`,
		period.MemberID, period.End, period.Start).Scan(&overlaps)
	if err != nil {
		return err
	}
	if overlaps {
		return errors.New("away period overlaps an existing period")
	}
	_, err = tx.Exec(ctx, `INSERT INTO member_away_periods
		(away_period_id, member_id, start_time, end_time, consent_version, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6)`, period.ID, period.MemberID,
		period.Start, period.End, period.ConsentVersion, period.CorrelationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'AWAY_PERIOD_SCHEDULED', 'member_away_period', $2,
		jsonb_build_object('start_time', $3::timestamptz, 'end_time', $4::timestamptz,
		'consent_version', $5::text), $6)`, period.MemberID, period.ID,
		period.Start, period.End, period.ConsentVersion, period.CorrelationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func awayPeriodByID(ctx context.Context, tx pgx.Tx, id string) (*AwayPeriod, error) {
	var period AwayPeriod
	var endID, endCorrelation *string
	err := tx.QueryRow(ctx, `SELECT away_period_id, member_id, start_time, end_time, consent_version,
		correlation_id, ended_at, end_idempotency_key, end_correlation_id
		FROM member_away_periods WHERE away_period_id = $1`, id).Scan(&period.ID, &period.MemberID,
		&period.Start, &period.End, &period.ConsentVersion, &period.CorrelationID,
		&period.EndedAt, &endID, &endCorrelation)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if endID != nil {
		period.EndID = *endID
	}
	if endCorrelation != nil {
		period.EndCorrelationID = *endCorrelation
	}
	return &period, nil
}

func sameAwayPeriod(previous, requested AwayPeriod) bool {
	return previous.ID == requested.ID && previous.MemberID == requested.MemberID &&
		previous.Start.Equal(requested.Start) && previous.End.Equal(requested.End) &&
		previous.ConsentVersion == requested.ConsentVersion && previous.CorrelationID == requested.CorrelationID
}

func (command EndAway) validate() error {
	if command.ID == "" || command.PeriodID == "" || command.MemberID == "" || command.At.IsZero() || command.CorrelationID == "" {
		return errors.New("away end requires identity, member, time, and correlation")
	}
	return nil
}

func (store *Store) EndAway(ctx context.Context, command EndAway) error {
	if err := command.validate(); err != nil {
		return err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "member-away:"+command.MemberID); err != nil {
		return err
	}
	period, err := awayPeriodByID(ctx, tx, command.PeriodID)
	if err != nil {
		return err
	}
	if period == nil || period.MemberID != command.MemberID {
		return errors.New("away period not found")
	}
	if period.EndedAt != nil {
		if period.EndID == command.ID && period.EndedAt.Equal(command.At) && period.EndCorrelationID == command.CorrelationID {
			return nil
		}
		return errors.New("away period was already ended by another command")
	}
	if command.At.Before(period.Start) || !command.At.Before(period.End) {
		return errors.New("away end must occur inside the period")
	}
	_, err = tx.Exec(ctx, `UPDATE member_away_periods SET ended_at = $1, end_idempotency_key = $2,
		end_correlation_id = $3 WHERE away_period_id = $4`, command.At, command.ID,
		command.CorrelationID, period.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'AWAY_PERIOD_ENDED', 'member_away_period', $2,
		jsonb_build_object('ended_at', $3::timestamptz), $4)`, period.MemberID, period.ID,
		command.At, command.CorrelationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
