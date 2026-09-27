package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func resolveReselection(ctx context.Context, tx pgx.Tx, choice Selection) (*Plan, string, error) {
	var id, market, catalog, plan, policy string
	var effective time.Time
	var expires *time.Time
	err := tx.QueryRow(ctx, `SELECT resilience_plan_id, market, catalog_version, member_plan_id, policy_version,
  effective_at, expires_at FROM resilience_plans WHERE member_id = $1
  ORDER BY effective_at DESC, resilience_plan_id DESC LIMIT 1 FOR UPDATE`, choice.MemberID).Scan(
		&id, &market, &catalog, &plan, &policy, &effective, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if effective.After(choice.EffectiveAt) {
		return nil, "", errors.New("selection effective time precedes the latest plan")
	}
	if expires != nil && !expires.After(choice.EffectiveAt) {
		return nil, "", nil
	}
	if market == choice.Market && catalog == choice.CatalogVersion && plan == choice.MemberPlanID && policy == choice.PolicyVersion {
		current, err := planByID(ctx, tx, id)
		return current, "", err
	}
	if !effective.Before(choice.EffectiveAt) {
		return nil, "", errors.New("a different plan is already effective at that time")
	}
	return nil, id, nil
}

func supersedeSelection(ctx context.Context, tx pgx.Tx, oldID string, choice Selection) error {
	if _, err := tx.Exec(ctx, `UPDATE resilience_plans SET expires_at = $1 WHERE resilience_plan_id = $2`, choice.EffectiveAt, oldID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
  VALUES ($1, 'RESILIENCE_PLAN_SUPERSEDED', 'resilience_plan', $2,
  jsonb_build_object('expires_at', $3::timestamptz, 'replacement_id', $4::text), $5)`,
		choice.MemberID, oldID, choice.EffectiveAt, choice.ID, choice.CorrelationID)
	return err
}
