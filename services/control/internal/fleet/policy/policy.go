package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Selection struct {
	ID               string
	MemberID         string
	Market           string
	CatalogVersion   string
	MemberPlanID     string
	PolicyVersion    string
	ConsentText      string
	ConsentVersion   string
	ExplanationShown string
	EffectiveAt      time.Time
	CorrelationID    string
}

type Plan struct {
	Selection
	DisplayName               string
	ReserveFloorPercent       float64
	EnergyMonthlyChargeCents  int64
	BatteryMonthlyChargeCents int64
	FlexibilityRewardCents    int64
}

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) Select(ctx context.Context, choice Selection) (*Plan, error) {
	if err := choice.validate(); err != nil {
		return nil, err
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "policy-selection:"+choice.ID); err != nil {
		return nil, err
	}
	previous, err := planByID(ctx, tx, choice.ID)
	if err != nil {
		return nil, err
	}
	if previous != nil {
		stored := previous.Selection
		if !stored.EffectiveAt.Equal(choice.EffectiveAt) {
			return nil, errors.New("selection idempotency key has different consent or plan")
		}
		stored.EffectiveAt = choice.EffectiveAt
		if stored != choice {
			return nil, errors.New("selection idempotency key has different consent or plan")
		}
		return previous, nil
	}
	plan, err := catalogPlan(ctx, tx, choice)
	if err != nil {
		return nil, err
	}
	var policyFloor float64
	err = tx.QueryRow(ctx, `SELECT member_plan_floor_percent FROM reserve_policies
		WHERE policy_version = $1 AND effective_at <= $2 AND (expires_at IS NULL OR expires_at > $2)`, choice.PolicyVersion, choice.EffectiveAt).Scan(&policyFloor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("reserve policy is not effective")
	}
	if err != nil {
		return nil, err
	}
	if policyFloor != plan.ReserveFloorPercent {
		return nil, errors.New("catalog reserve does not match effective reserve policy")
	}
	_, err = tx.Exec(ctx, `INSERT INTO resilience_plans
		(resilience_plan_id, member_id, market, reserve_floor_percent, consent_text, consent_version,
		policy_version, effective_at, correlation_id, catalog_version, member_plan_id, explanation_shown)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		choice.ID, choice.MemberID, choice.Market, plan.ReserveFloorPercent, choice.ConsentText, choice.ConsentVersion,
		choice.PolicyVersion, choice.EffectiveAt, choice.CorrelationID, choice.CatalogVersion, choice.MemberPlanID, choice.ExplanationShown)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'RESILIENCE_PLAN_SELECTED', 'resilience_plan', $2,
		jsonb_build_object('catalog_version', $3::text, 'member_plan_id', $4::text, 'policy_version', $5::text, 'effective_at', $6::timestamptz), $7)`,
		choice.MemberID, choice.ID, choice.CatalogVersion, choice.MemberPlanID, choice.PolicyVersion, choice.EffectiveAt, choice.CorrelationID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return plan, nil
}

func (choice Selection) validate() error {
	if choice.ID == "" || choice.MemberID == "" || choice.Market == "" || choice.CatalogVersion == "" || choice.MemberPlanID == "" || choice.PolicyVersion == "" || choice.CorrelationID == "" || choice.EffectiveAt.IsZero() {
		return errors.New("selection identity, catalog, policy, and effective time are required")
	}
	if strings.TrimSpace(choice.ConsentText) == "" || strings.TrimSpace(choice.ConsentVersion) == "" || strings.TrimSpace(choice.ExplanationShown) == "" {
		return errors.New("consent text, version, and explanation shown are required")
	}
	return nil
}

func catalogPlan(ctx context.Context, tx pgx.Tx, choice Selection) (*Plan, error) {
	plan := &Plan{Selection: choice}
	var name *string
	var reserve *float64
	err := tx.QueryRow(ctx, `SELECT display_name, reserve_floor_percent, energy_monthly_charge_cents, battery_monthly_charge_cents, flexibility_reward_cents
		FROM pricing_catalog_snapshots WHERE catalog_version = $1 AND member_plan_id = $2 AND market = $3
		AND effective_at <= $4 AND (expires_at IS NULL OR expires_at > $4)`, choice.CatalogVersion, choice.MemberPlanID, choice.Market, choice.EffectiveAt).
		Scan(&name, &reserve, &plan.EnergyMonthlyChargeCents, &plan.BatteryMonthlyChargeCents, &plan.FlexibilityRewardCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("catalog plan is not effective")
	}
	if err != nil {
		return nil, err
	}
	if name == nil || reserve == nil {
		return nil, errors.New("catalog plan has no name or reserve band")
	}
	plan.DisplayName, plan.ReserveFloorPercent = *name, *reserve
	return plan, nil
}

func (store *Store) Current(ctx context.Context, memberID string, at time.Time) (*Plan, error) {
	row := store.pool.QueryRow(ctx, `SELECT selection.resilience_plan_id, selection.member_id, selection.market,
		selection.catalog_version, selection.member_plan_id, selection.policy_version, selection.consent_text,
		selection.consent_version, selection.explanation_shown, selection.effective_at, selection.correlation_id,
		catalog.display_name, selection.reserve_floor_percent, catalog.energy_monthly_charge_cents,
		catalog.battery_monthly_charge_cents, catalog.flexibility_reward_cents
		FROM resilience_plans AS selection JOIN pricing_catalog_snapshots AS catalog
		ON catalog.catalog_version = selection.catalog_version AND catalog.member_plan_id = selection.member_plan_id
		WHERE selection.member_id = $1 AND selection.effective_at <= $2
		AND (selection.expires_at IS NULL OR selection.expires_at > $2)
		AND selection.consent_text <> '' AND selection.consent_version <> '' AND selection.explanation_shown <> ''
		ORDER BY selection.effective_at DESC, selection.resilience_plan_id DESC LIMIT 1`, memberID, at)
	plan, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return plan, err
}

func planByID(ctx context.Context, tx pgx.Tx, id string) (*Plan, error) {
	row := tx.QueryRow(ctx, `SELECT selection.resilience_plan_id, selection.member_id, selection.market,
		selection.catalog_version, selection.member_plan_id, selection.policy_version, selection.consent_text,
		selection.consent_version, selection.explanation_shown, selection.effective_at, selection.correlation_id,
		catalog.display_name, selection.reserve_floor_percent, catalog.energy_monthly_charge_cents,
		catalog.battery_monthly_charge_cents, catalog.flexibility_reward_cents
		FROM resilience_plans AS selection JOIN pricing_catalog_snapshots AS catalog
		ON catalog.catalog_version = selection.catalog_version AND catalog.member_plan_id = selection.member_plan_id
		WHERE selection.resilience_plan_id = $1`, id)
	plan, err := scanPlan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return plan, err
}

func scanPlan(row pgx.Row) (*Plan, error) {
	plan := &Plan{}
	err := row.Scan(&plan.ID, &plan.MemberID, &plan.Market, &plan.CatalogVersion, &plan.MemberPlanID,
		&plan.PolicyVersion, &plan.ConsentText, &plan.ConsentVersion, &plan.ExplanationShown,
		&plan.EffectiveAt, &plan.CorrelationID, &plan.DisplayName, &plan.ReserveFloorPercent,
		&plan.EnergyMonthlyChargeCents, &plan.BatteryMonthlyChargeCents, &plan.FlexibilityRewardCents)
	if err != nil {
		return nil, fmt.Errorf("load selected plan: %w", err)
	}
	plan.EffectiveAt = plan.EffectiveAt.UTC()
	return plan, nil
}
