package policy

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type OverrideReason string

const (
	OverrideWeather        OverrideReason = "WEATHER"
	OverrideOutageRisk     OverrideReason = "OUTAGE_RISK"
	OverrideHealth         OverrideReason = "HEALTH"
	OverrideStaleTelemetry OverrideReason = "STALE_TELEMETRY"
	OverrideAlarm          OverrideReason = "ALARM"
	OverrideCommunications OverrideReason = "COMMUNICATIONS"
)

type ReserveOverride struct {
	ID            string
	MemberID      string
	Reason        OverrideReason
	FloorPercent  float64
	EffectiveAt   time.Time
	ExpiresAt     time.Time
	PolicyVersion string
	EvidenceID    string
	CorrelationID string
}

func (command ReserveOverride) validate() error {
	if command.ID == "" || command.MemberID == "" || command.PolicyVersion == "" || command.EvidenceID == "" || command.CorrelationID == "" || command.EffectiveAt.IsZero() || !command.ExpiresAt.After(command.EffectiveAt) {
		return errors.New("override identity, evidence, policy, and bounded interval are required")
	}
	if math.IsNaN(command.FloorPercent) || math.IsInf(command.FloorPercent, 0) || command.FloorPercent < 0 || command.FloorPercent > 100 {
		return errors.New("override floor must be a finite percent")
	}
	switch command.Reason {
	case OverrideWeather, OverrideOutageRisk, OverrideHealth, OverrideStaleTelemetry, OverrideAlarm, OverrideCommunications:
		return nil
	default:
		return errors.New("unsupported override reason")
	}
}

func (store *Store) ApplyOverride(ctx context.Context, command ReserveOverride) error {
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
	retry, err := existingOverride(ctx, tx, command)
	if err != nil {
		return err
	}
	if retry {
		return nil
	}
	policyVersion, effective, err := currentReserveForOverride(ctx, tx, command.MemberID, command.EffectiveAt)
	if err != nil {
		return err
	}
	if policyVersion != command.PolicyVersion || command.FloorPercent <= effective {
		return errors.New("override must raise the current policy reserve")
	}
	_, err = tx.Exec(ctx, `INSERT INTO reserve_overrides
		(reserve_override_id, member_id, reason, reserve_floor_percent, effective_at, expires_at, policy_version, evidence_id, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, command.ID, command.MemberID, command.Reason,
		command.FloorPercent, command.EffectiveAt, command.ExpiresAt, command.PolicyVersion, command.EvidenceID, command.CorrelationID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'RESERVE_OVERRIDE_APPLIED', 'reserve_override', $2,
		jsonb_build_object('reason', $3::text, 'reserve_floor_percent', $4::double precision,
		'effective_at', $5::timestamptz, 'expires_at', $6::timestamptz, 'evidence_id', $7::text), $8)`,
		command.MemberID, command.ID, command.Reason, command.FloorPercent, command.EffectiveAt,
		command.ExpiresAt, command.EvidenceID, command.CorrelationID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentReserveForOverride(ctx context.Context, tx pgx.Tx, memberID string, at time.Time) (string, float64, error) {
	var policyVersion string
	var effective float64
	err := tx.QueryRow(ctx, `SELECT plan.policy_version, GREATEST(
		COALESCE((SELECT w.temporary_reserve_percent FROM travel_flex_windows w
			WHERE w.member_id = plan.member_id AND w.policy_version = plan.policy_version
			AND w.start_time <= $2 AND w.end_time > $2
			AND (w.cancelled_at IS NULL OR w.cancelled_at > $2)
			ORDER BY w.start_time DESC LIMIT 1), plan.reserve_floor_percent),
		r.protected_hardware_floor_percent, r.dynamic_override_percent,
		COALESCE((SELECT max(o.reserve_floor_percent) FROM reserve_overrides o
			WHERE o.member_id = plan.member_id AND o.effective_at <= $2
			AND (o.expires_at IS NULL OR o.expires_at > $2)), 0))
		FROM resilience_plans plan JOIN reserve_policies r ON r.policy_version = plan.policy_version
		WHERE plan.member_id = $1 AND plan.effective_at <= $2 AND (plan.expires_at IS NULL OR plan.expires_at > $2)
		AND r.effective_at <= $2 AND (r.expires_at IS NULL OR r.expires_at > $2)
		ORDER BY plan.effective_at DESC, plan.resilience_plan_id DESC LIMIT 1`, memberID, at).Scan(&policyVersion, &effective)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, errors.New("consented effective plan is required")
	}
	if err != nil {
		return "", 0, err
	}
	return policyVersion, effective, nil
}

func existingOverride(ctx context.Context, tx pgx.Tx, command ReserveOverride) (bool, error) {
	var previous ReserveOverride
	err := tx.QueryRow(ctx, `SELECT member_id, reason, reserve_floor_percent, effective_at, expires_at, policy_version, evidence_id, correlation_id
		FROM reserve_overrides WHERE reserve_override_id = $1`, command.ID).Scan(
		&previous.MemberID, &previous.Reason, &previous.FloorPercent, &previous.EffectiveAt, &previous.ExpiresAt,
		&previous.PolicyVersion, &previous.EvidenceID, &previous.CorrelationID)
	if err == nil {
		if previous.MemberID == command.MemberID && previous.Reason == command.Reason && previous.FloorPercent == command.FloorPercent &&
			previous.EffectiveAt.Equal(command.EffectiveAt) && previous.ExpiresAt.Equal(command.ExpiresAt) &&
			previous.PolicyVersion == command.PolicyVersion && previous.EvidenceID == command.EvidenceID && previous.CorrelationID == command.CorrelationID {
			return true, nil
		}
		return false, errors.New("override idempotency key has different content")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	return false, nil
}
