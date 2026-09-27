package policy

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type AnomalyPreference struct {
	ID              string
	MemberID        string
	OptIn           bool
	ConsentText     string
	ConsentVersion  string
	BaselineUpperKW float64
	BaselineBegin   time.Time
	BaselineEnd     time.Time
	EffectiveAt     time.Time
	ExpiresAt       time.Time
	CorrelationID   string
}

func (choice AnomalyPreference) validate() error {
	if choice.ID == "" || choice.MemberID == "" || choice.ConsentText == "" || choice.ConsentVersion == "" || choice.CorrelationID == "" || choice.BaselineBegin.IsZero() || !choice.BaselineEnd.After(choice.BaselineBegin) || choice.EffectiveAt.IsZero() || !choice.ExpiresAt.After(choice.EffectiveAt) {
		return errors.New("anomaly preference requires consent, baseline interval, and effective period")
	}
	if math.IsNaN(choice.BaselineUpperKW) || math.IsInf(choice.BaselineUpperKW, 0) || choice.BaselineUpperKW < 0 || choice.BaselineUpperKW >= 1000000 {
		return errors.New("baseline upper bound must be finite and nonnegative")
	}
	return nil
}

func (store *Store) SetAnomalyPreference(ctx context.Context, choice AnomalyPreference) error {
	if err := choice.validate(); err != nil {
		return err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "member-anomaly:"+choice.MemberID); err != nil {
		return err
	}
	previous, err := anomalyPreferenceByID(ctx, tx, choice.ID)
	if err != nil {
		return err
	}
	if previous != nil {
		if sameAnomalyPreference(*previous, choice) {
			return nil
		}
		return errors.New("preference idempotency key has different consent or baseline")
	}
	_, err = tx.Exec(ctx, `INSERT INTO member_anomaly_preferences
		(preference_id, member_id, opted_in, consent_text, consent_version, baseline_upper_kw,
		baseline_begin, baseline_end, effective_at, expires_at, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		choice.ID, choice.MemberID, choice.OptIn, choice.ConsentText, choice.ConsentVersion, choice.BaselineUpperKW,
		choice.BaselineBegin, choice.BaselineEnd, choice.EffectiveAt, choice.ExpiresAt, choice.CorrelationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'ANOMALY_PREFERENCE_SET', 'member_anomaly_preference', $2,
		jsonb_build_object('opted_in', $3::boolean, 'consent_version', $4::text, 'baseline_upper_kw', $5::double precision,
		'effective_at', $6::timestamptz, 'expires_at', $7::timestamptz), $8)`,
		choice.MemberID, choice.ID, choice.OptIn, choice.ConsentVersion, choice.BaselineUpperKW,
		choice.EffectiveAt, choice.ExpiresAt, choice.CorrelationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func anomalyPreferenceByID(ctx context.Context, tx pgx.Tx, id string) (*AnomalyPreference, error) {
	var choice AnomalyPreference
	err := tx.QueryRow(ctx, `SELECT preference_id, member_id, opted_in, consent_text, consent_version,
		baseline_upper_kw, baseline_begin, baseline_end, effective_at, expires_at, correlation_id
		FROM member_anomaly_preferences WHERE preference_id = $1`, id).Scan(&choice.ID, &choice.MemberID,
		&choice.OptIn, &choice.ConsentText, &choice.ConsentVersion, &choice.BaselineUpperKW,
		&choice.BaselineBegin, &choice.BaselineEnd, &choice.EffectiveAt, &choice.ExpiresAt, &choice.CorrelationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &choice, err
}

func sameAnomalyPreference(previous, requested AnomalyPreference) bool {
	if !previous.BaselineBegin.Equal(requested.BaselineBegin) || !previous.BaselineEnd.Equal(requested.BaselineEnd) || !previous.EffectiveAt.Equal(requested.EffectiveAt) || !previous.ExpiresAt.Equal(requested.ExpiresAt) {
		return false
	}
	previous.BaselineBegin, previous.BaselineEnd = requested.BaselineBegin, requested.BaselineEnd
	previous.EffectiveAt, previous.ExpiresAt = requested.EffectiveAt, requested.ExpiresAt
	return previous == requested
}
