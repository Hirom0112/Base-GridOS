package policy

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReselectFormerPlanSupersedesCurrentAndNoOpsCurrent(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	_, err := pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots(catalog_version,member_plan_id,market,display_name,reserve_floor_percent,energy_plan,energy_term_months,energy_monthly_charge_cents,battery_plan,battery_term_months,battery_monthly_charge_cents,flexibility_reward_cents,effective_at,correlation_id) VALUES ('catalog-v3','plan-cedar','TX','Cedar Return',65,'{}',0,1999,'{}',0,1500,500,$1,'fixture')`, begin.Add(24*time.Hour))
	require.NoError(t, err)
	store := New(pool)
	first := Selection{ID: "select-first", MemberID: "member-return", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I accept Cedar", ConsentVersion: "v1", ExplanationShown: "Reserve explained", EffectiveAt: begin, CorrelationID: "first"}
	firstPlan, err := selectWithOffer(t, store, first)
	require.NoError(t, err)
	second := first
	second.ID = "select-second"
	second.CatalogVersion = "catalog-v2"
	second.PolicyVersion = "policy-v2"
	second.EffectiveAt = begin.Add(25 * time.Hour)
	second.CorrelationID = "second"
	secondPlan, err := selectWithOffer(t, store, second)
	require.NoError(t, err)
	third := first
	third.ID = "select-third"
	third.CatalogVersion = "catalog-v3"
	third.EffectiveAt = begin.Add(26 * time.Hour)
	third.CorrelationID = "third"
	thirdPlan, err := selectWithOffer(t, store, third)
	require.NoError(t, err)
	require.Equal(t, "select-third", thirdPlan.ID)
	current, err := store.Current(ctx, third.MemberID, third.EffectiveAt)
	require.NoError(t, err)
	require.Equal(t, thirdPlan.ID, current.ID)
	for _, item := range []struct {
		id      string
		expires *time.Time
	}{{firstPlan.ID, &second.EffectiveAt}, {secondPlan.ID, &third.EffectiveAt}, {thirdPlan.ID, nil}} {
		var expires *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT expires_at FROM resilience_plans WHERE resilience_plan_id=$1`, item.id).Scan(&expires))
		if item.expires == nil {
			require.Nil(t, expires)
		} else {
			require.True(t, item.expires.Equal(*expires))
		}
	}
	same := third
	same.ID = "select-same-again"
	same.EffectiveAt = begin.Add(27 * time.Hour)
	same.CorrelationID = "same"
	noOp, err := selectWithOffer(t, store, same)
	require.NoError(t, err)
	require.Equal(t, thirdPlan.ID, noOp.ID)
	var selected, superseded int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE actor_id=$1 AND action='RESILIENCE_PLAN_SELECTED'`, first.MemberID).Scan(&selected))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE actor_id=$1 AND action='RESILIENCE_PLAN_SUPERSEDED'`, first.MemberID).Scan(&superseded))
	require.Equal(t, 3, selected)
	require.Equal(t, 2, superseded)
	conflict := second
	conflict.ID = "select-same-time-different"
	conflict.EffectiveAt = third.EffectiveAt
	conflict.CorrelationID = "conflict"
	_, err = selectWithOffer(t, store, conflict)
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "SQLSTATE"), err)
}

func TestReselectMigrationRollsBackAndReapplies(t *testing.T) {
	pool := policyDatabase(t)
	rollback, err := os.ReadFile("../../../../../database/rollback/0019_reselect_plans.sql")
	require.NoError(t, err)
	forward, err := os.ReadFile("../../../../../database/migrations/0019_reselect_plans.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
}
