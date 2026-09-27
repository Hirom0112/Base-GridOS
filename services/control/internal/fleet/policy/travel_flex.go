package policy

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type EarlyReturnAction string

const (
	RestorePlanReserve    EarlyReturnAction = "RESTORE_PLAN_RESERVE"
	RestoreMaximumReserve EarlyReturnAction = "RESTORE_MAXIMUM_RESERVE"
)

type CreditType string

const (
	FixedDaily  CreditType = "FIXED_DAILY"
	FixedEvent  CreditType = "FIXED_EVENT"
	FixedAnnual CreditType = "FIXED_ANNUAL"
)

type TravelFlex struct {
	ID                      string
	OfferID                 string
	MemberID                string
	Start                   time.Time
	End                     time.Time
	Timezone                string
	TemporaryReservePercent float64
	EarlyReturnAction       EarlyReturnAction
	CreditType              CreditType
	CreditCents             int64
	ConsentText             string
	ConsentVersion          string
	PolicyVersion           string
	CorrelationID           string
	CancelledAt             *time.Time
	EndID                   string
	EndCorrelationID        string
}

type EarlyReturn struct {
	ID            string
	WindowID      string
	MemberID      string
	At            time.Time
	CorrelationID string
}

func (window TravelFlex) validateWindow() error {
	if window.ID == "" || window.OfferID == "" || window.MemberID == "" || window.PolicyVersion == "" || window.CorrelationID == "" || window.Start.IsZero() || !window.End.After(window.Start) {
		return errors.New("travel window identity, policy, and ordered times are required")
	}
	zone, err := time.LoadLocation(window.Timezone)
	if err != nil || window.Start.Location().String() != zone.String() || window.End.Location().String() != zone.String() {
		return errors.New("travel window requires a valid local timezone")
	}
	return nil
}

func (window TravelFlex) validateTerms() error {
	if math.IsNaN(window.TemporaryReservePercent) || math.IsInf(window.TemporaryReservePercent, 0) || window.TemporaryReservePercent < 0 || window.TemporaryReservePercent > 100 {
		return errors.New("temporary reserve percent is invalid")
	}
	if window.EarlyReturnAction != RestorePlanReserve && window.EarlyReturnAction != RestoreMaximumReserve {
		return errors.New("early return action is invalid")
	}
	if window.CreditType != FixedDaily && window.CreditType != FixedEvent && window.CreditType != FixedAnnual || window.CreditCents < 0 {
		return errors.New("fixed credit is invalid")
	}
	if strings.TrimSpace(window.ConsentText) == "" || strings.TrimSpace(window.ConsentVersion) == "" {
		return errors.New("travel window consent is required")
	}
	return nil
}

func (store *Store) ScheduleTravelFlex(ctx context.Context, window TravelFlex) (*TravelFlex, error) {
	if err := window.validateWindow(); err != nil {
		return nil, err
	}
	if err := window.validateTerms(); err != nil {
		return nil, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "member-policy:"+window.MemberID); err != nil {
		return nil, err
	}
	existing, err := travelWindow(ctx, tx, window.ID, window.MemberID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !sameTravelWindow(*existing, window) {
			return nil, errors.New("travel window idempotency key has different terms")
		}
		return existing, nil
	}
	plan, err := store.Current(ctx, window.MemberID, window.Start)
	if err != nil {
		return nil, err
	}
	if plan == nil || plan.PolicyVersion != window.PolicyVersion || window.TemporaryReservePercent > plan.ReserveFloorPercent {
		return nil, errors.New("travel window requires a consented plan and no higher reserve")
	}
	if err = requireTravelOffer(ctx, tx, window, plan); err != nil {
		return nil, err
	}
	var overlaps bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM travel_flex_windows
		WHERE member_id = $1 AND start_time < $2 AND COALESCE(cancelled_at, end_time) > $3)`, window.MemberID, window.End, window.Start).Scan(&overlaps)
	if err != nil {
		return nil, err
	}
	if overlaps {
		return nil, errors.New("travel window overlaps an existing window")
	}
	_, err = tx.Exec(ctx, `INSERT INTO travel_flex_windows
		(travel_flex_window_id, member_id, start_time, end_time, timezone, temporary_reserve_percent,
		early_return_action, credit_type, credit_cents, consent_text, consent_version, policy_version, correlation_id, offer_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		window.ID, window.MemberID, window.Start, window.End, window.Timezone, window.TemporaryReservePercent,
		window.EarlyReturnAction, window.CreditType, window.CreditCents, window.ConsentText, window.ConsentVersion, window.PolicyVersion, window.CorrelationID, window.OfferID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'TRAVEL_FLEX_SCHEDULED', 'travel_flex_window', $2,
		jsonb_build_object('start_time', $3::timestamptz, 'end_time', $4::timestamptz, 'timezone', $5::text,
		'credit_type', $6::text, 'credit_cents', $7::bigint, 'offer_id', $8::text), $9)`,
		window.MemberID, window.ID, window.Start, window.End, window.Timezone, window.CreditType, window.CreditCents, window.OfferID, window.CorrelationID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &window, nil
}

func requireTravelOffer(ctx context.Context, tx pgx.Tx, window TravelFlex, plan *Plan) error {
	offer, err := presentedOffer(ctx, tx, window.OfferID)
	if err != nil {
		return err
	}
	if offer == nil || offer.Kind != TravelFlexOffer || offer.MemberID != window.MemberID ||
		offer.Market != plan.Market || offer.CatalogVersion != plan.CatalogVersion ||
		offer.MemberPlanID != plan.MemberPlanID || offer.TemporaryReservePercent == nil ||
		*offer.TemporaryReservePercent != window.TemporaryReservePercent ||
		offer.CreditType != window.CreditType || offer.CreditCents != window.CreditCents ||
		offer.ConsentText != window.ConsentText || offer.ConsentVersion != window.ConsentVersion ||
		window.Start.Before(offer.EffectiveAt) || !window.Start.Before(offer.ExpiresAt) {
		return errors.New("travel window does not match presented offer")
	}
	return nil
}

func sameTravelWindow(stored, requested TravelFlex) bool {
	return stored.ID == requested.ID && stored.OfferID == requested.OfferID && stored.MemberID == requested.MemberID && stored.Start.Equal(requested.Start) && stored.End.Equal(requested.End) &&
		stored.Timezone == requested.Timezone && stored.TemporaryReservePercent == requested.TemporaryReservePercent &&
		stored.EarlyReturnAction == requested.EarlyReturnAction && stored.CreditType == requested.CreditType &&
		stored.CreditCents == requested.CreditCents && stored.ConsentText == requested.ConsentText &&
		stored.ConsentVersion == requested.ConsentVersion && stored.PolicyVersion == requested.PolicyVersion && stored.CorrelationID == requested.CorrelationID
}

func travelWindow(ctx context.Context, tx pgx.Tx, id, memberID string) (*TravelFlex, error) {
	window := &TravelFlex{}
	var endKey, endCorrelation, offerID *string
	err := tx.QueryRow(ctx, `SELECT travel_flex_window_id, member_id, start_time, end_time, timezone,
		temporary_reserve_percent, early_return_action, credit_type, credit_cents, consent_text,
		consent_version, policy_version, correlation_id, cancelled_at, end_idempotency_key, end_correlation_id, offer_id
		FROM travel_flex_windows WHERE travel_flex_window_id = $1 AND member_id = $2`, id, memberID).
		Scan(&window.ID, &window.MemberID, &window.Start, &window.End, &window.Timezone,
			&window.TemporaryReservePercent, &window.EarlyReturnAction, &window.CreditType, &window.CreditCents,
			&window.ConsentText, &window.ConsentVersion, &window.PolicyVersion, &window.CorrelationID,
			&window.CancelledAt, &endKey, &endCorrelation, &offerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if endKey != nil {
		window.EndID = *endKey
	}
	if endCorrelation != nil {
		window.EndCorrelationID = *endCorrelation
	}
	if offerID != nil {
		window.OfferID = *offerID
	}
	window.Start, window.End = window.Start.UTC(), window.End.UTC()
	if window.CancelledAt != nil {
		at := window.CancelledAt.UTC()
		window.CancelledAt = &at
	}
	return window, nil
}

func (store *Store) EndTravelFlexEarly(ctx context.Context, command EarlyReturn) error {
	if err := command.validate(); err != nil {
		return err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "member-policy:"+command.MemberID); err != nil {
		return err
	}
	window, err := travelWindow(ctx, tx, command.WindowID, command.MemberID)
	if err != nil {
		return err
	}
	if window == nil {
		return errors.New("travel window not found")
	}
	if window.CancelledAt != nil {
		if window.EndID == command.ID && window.EndCorrelationID == command.CorrelationID && window.CancelledAt.Equal(command.At) {
			return nil
		}
		return errors.New("travel window was already ended by another command")
	}
	if command.At.Before(window.Start) || !command.At.Before(window.End) {
		return errors.New("early return must occur inside the travel window")
	}
	if window.EarlyReturnAction == RestoreMaximumReserve {
		if err = store.maximumReturn(ctx, tx, *window, command); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE travel_flex_windows SET cancelled_at = $1, end_idempotency_key = $2,
		end_correlation_id = $3 WHERE travel_flex_window_id = $4`, command.At, command.ID, command.CorrelationID, window.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, previous_values, new_values, correlation_id)
		VALUES ($1, 'TRAVEL_FLEX_CANCELLED', 'travel_flex_window', $2,
		jsonb_build_object('end_time', $3::timestamptz), jsonb_build_object('cancelled_at', $4::timestamptz, 'action', $5::text), $6)`,
		window.MemberID, window.ID, window.End, command.At, window.EarlyReturnAction, command.CorrelationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (command EarlyReturn) validate() error {
	if command.ID == "" || command.WindowID == "" || command.MemberID == "" || command.CorrelationID == "" || command.At.IsZero() {
		return errors.New("early return identity, window, member, time, and correlation are required")
	}
	return nil
}

func (store *Store) maximumReturn(ctx context.Context, tx pgx.Tx, window TravelFlex, command EarlyReturn) error {
	plan, err := store.Current(ctx, window.MemberID, command.At)
	if err != nil {
		return err
	}
	if plan == nil {
		return errors.New("current plan required for maximum reserve")
	}
	var maximum *float64
	err = tx.QueryRow(ctx, `SELECT max(reserve_floor_percent) FROM pricing_catalog_snapshots
		WHERE catalog_version = $1 AND market = $2 AND reserve_floor_percent IS NOT NULL`, plan.CatalogVersion, plan.Market).Scan(&maximum)
	if err != nil {
		return err
	}
	if maximum == nil || *maximum < plan.ReserveFloorPercent {
		return errors.New("catalog maximum reserve is unavailable")
	}
	_, err = tx.Exec(ctx, `INSERT INTO reserve_overrides
		(reserve_override_id, member_id, reason, reserve_floor_percent, effective_at, expires_at, policy_version, correlation_id)
		VALUES ($1, $2, 'EARLY_RETURN', $3, $4, $5, $6, $7)`,
		window.ID+":early-return", window.MemberID, *maximum, command.At, window.End, plan.PolicyVersion, command.CorrelationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'RESERVE_OVERRIDE_APPLIED', 'reserve_override', $2,
		jsonb_build_object('reason', 'EARLY_RETURN', 'reserve_floor_percent', $3::double precision,
		'expires_at', $4::timestamptz), $5)`, window.MemberID, window.ID+":early-return", *maximum, window.End, command.CorrelationID)
	return err
}
