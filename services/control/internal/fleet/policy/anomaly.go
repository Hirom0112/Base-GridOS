package policy

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MeasuredSiteLoad struct {
	ObservationID string
	SiteID        string
	ObservedAt    time.Time
	ToHomeKW      float64
	CorrelationID string
}

type AnomalyAlert struct {
	ID            string
	MemberID      string
	Text          string
	PreferenceID  string
	ObservationID string
	SiteID        string
	ObservedAt    time.Time
	LoadKW        float64
}

func (reading MeasuredSiteLoad) validate() error {
	if reading.ObservationID == "" || reading.SiteID == "" || reading.CorrelationID == "" || reading.ObservedAt.IsZero() || math.IsNaN(reading.ToHomeKW) || math.IsInf(reading.ToHomeKW, 0) || reading.ToHomeKW < 0 {
		return errors.New("measured site load requires identity, time, and finite nonnegative kW")
	}
	return nil
}

func (store *Store) EvaluateAnomaly(ctx context.Context, reading MeasuredSiteLoad) (*AnomalyAlert, error) {
	if err := reading.validate(); err != nil {
		return nil, err
	}
	var memberID string
	err := store.pool.QueryRow(ctx, `SELECT member_id FROM member_sites WHERE site_id = $1`, reading.SiteID).Scan(&memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("site has no authorized member binding")
	}
	if err != nil {
		return nil, err
	}
	preference, err := currentAnomalyPreference(ctx, store.pool, memberID, reading.ObservedAt)
	if err != nil || preference == nil {
		return nil, err
	}
	if !preference.OptIn || reading.ObservedAt.Before(preference.BaselineBegin) || !reading.ObservedAt.Before(preference.BaselineEnd) || reading.ToHomeKW <= preference.BaselineUpperKW {
		return nil, nil
	}
	away, err := memberIsAway(ctx, store.pool, memberID, reading.ObservedAt)
	if err != nil || !away {
		return nil, err
	}
	alert := AnomalyAlert{ID: memberID + ":" + preference.ID + ":" + reading.ObservationID, MemberID: memberID,
		Text: "energy anomaly signal", PreferenceID: preference.ID, ObservationID: reading.ObservationID,
		SiteID: reading.SiteID, ObservedAt: reading.ObservedAt.UTC(), LoadKW: reading.ToHomeKW}
	return store.recordAnomalyAlert(ctx, alert, reading, *preference)
}

func currentAnomalyPreference(ctx context.Context, pool *pgxpool.Pool, memberID string, at time.Time) (*AnomalyPreference, error) {
	var choice AnomalyPreference
	err := pool.QueryRow(ctx, `SELECT preference_id, member_id, opted_in, consent_text, consent_version,
		baseline_upper_kw, baseline_begin, baseline_end, effective_at, expires_at, correlation_id
		FROM member_anomaly_preferences WHERE member_id = $1 AND effective_at <= $2 AND expires_at > $2
		ORDER BY effective_at DESC, preference_id DESC LIMIT 1`, memberID, at).Scan(&choice.ID,
		&choice.MemberID, &choice.OptIn, &choice.ConsentText, &choice.ConsentVersion,
		&choice.BaselineUpperKW, &choice.BaselineBegin, &choice.BaselineEnd,
		&choice.EffectiveAt, &choice.ExpiresAt, &choice.CorrelationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &choice, err
}

func memberIsAway(ctx context.Context, pool *pgxpool.Pool, memberID string, at time.Time) (bool, error) {
	var away bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM member_away_periods WHERE member_id = $1 AND start_time <= $2
		AND end_time > $2 AND (ended_at IS NULL OR ended_at > $2)
		UNION ALL
		SELECT 1 FROM travel_flex_windows w JOIN flexibility_offers f
		ON f.offer_id = w.offer_id AND f.offer_type = 'TRAVEL_FLEX' AND f.member_id = w.member_id
		WHERE w.member_id = $1 AND w.start_time <= $2 AND w.end_time > $2
		AND (w.cancelled_at IS NULL OR w.cancelled_at > $2))`, memberID, at).Scan(&away)
	return away, err
}

func (store *Store) recordAnomalyAlert(ctx context.Context, alert AnomalyAlert, reading MeasuredSiteLoad, preference AnomalyPreference) (*AnomalyAlert, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "member-alert:"+alert.ID); err != nil {
		return nil, err
	}
	previous, err := anomalyAlertByID(ctx, tx, alert.ID)
	if err != nil {
		return nil, err
	}
	if previous != nil {
		if *previous == alert {
			return previous, nil
		}
		return nil, errors.New("observation idempotency key has different anomaly evidence")
	}
	_, err = tx.Exec(ctx, `INSERT INTO member_alerts
		(alert_id, member_id, kind, message, preference_id, evidence, observed_at, correlation_id)
		VALUES ($1, $2, 'ENERGY_ANOMALY_SIGNAL', 'energy anomaly signal', $3,
		jsonb_build_object('observation_id', $4::text, 'site_id', $5::text, 'to_home_kw', $6::double precision,
		'baseline_upper_kw', $7::double precision, 'consent_version', $8::text,
		'baseline_begin', $9::timestamptz, 'baseline_end', $10::timestamptz), $11, $12)`,
		alert.ID, alert.MemberID, preference.ID, reading.ObservationID, reading.SiteID,
		reading.ToHomeKW, preference.BaselineUpperKW, preference.ConsentVersion,
		preference.BaselineBegin, preference.BaselineEnd, reading.ObservedAt, reading.CorrelationID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'ENERGY_ANOMALY_SIGNAL', 'member_alert', $2,
		jsonb_build_object('preference_id', $3::text, 'observation_id', $4::text, 'site_id', $5::text), $6)`,
		alert.MemberID, alert.ID, preference.ID, reading.ObservationID, reading.SiteID, reading.CorrelationID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &alert, nil
}

func anomalyAlertByID(ctx context.Context, tx pgx.Tx, id string) (*AnomalyAlert, error) {
	var alert AnomalyAlert
	err := tx.QueryRow(ctx, `SELECT alert_id, member_id, message, preference_id,
		evidence->>'observation_id', evidence->>'site_id', observed_at,
		(evidence->>'to_home_kw')::double precision FROM member_alerts WHERE alert_id = $1`, id).Scan(
		&alert.ID, &alert.MemberID, &alert.Text, &alert.PreferenceID,
		&alert.ObservationID, &alert.SiteID, &alert.ObservedAt, &alert.LoadKW)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	alert.ObservedAt = alert.ObservedAt.UTC()
	return &alert, nil
}
