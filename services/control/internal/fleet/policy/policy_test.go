package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPlanSelectionUsesConsentedEffectiveCatalog(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	selection := Selection{
		ID: "selection-1", MemberID: "member-1", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1",
		ConsentText: "I accept the Cedar reserve and reward", ConsentVersion: "consent-v1", ExplanationShown: "A larger backup reserve limits dispatch",
		EffectiveAt: begin.Add(time.Hour), CorrelationID: "correlation-1",
	}
	_, err := store.Select(ctx, Selection{ID: "unconsented", MemberID: selection.MemberID, Market: selection.Market, CatalogVersion: selection.CatalogVersion, MemberPlanID: selection.MemberPlanID, PolicyVersion: selection.PolicyVersion, EffectiveAt: selection.EffectiveAt, CorrelationID: selection.CorrelationID})
	require.Error(t, err)
	before, err := store.Current(ctx, selection.MemberID, selection.EffectiveAt.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.Nil(t, before)
	chosen, err := store.Select(ctx, selection)
	require.NoError(t, err)
	require.Equal(t, "Cedar", chosen.DisplayName)
	require.Equal(t, 65.0, chosen.ReserveFloorPercent)
	require.Equal(t, int64(1999), chosen.EnergyMonthlyChargeCents)
	require.Equal(t, int64(1500), chosen.BatteryMonthlyChargeCents)
	require.Equal(t, int64(500), chosen.FlexibilityRewardCents)
	retried, err := store.Select(ctx, selection)
	require.NoError(t, err)
	require.Equal(t, chosen, retried)
	changed := selection
	changed.ConsentText = "different consent"
	_, err = store.Select(ctx, changed)
	require.Error(t, err)
	before, err = store.Current(ctx, selection.MemberID, selection.EffectiveAt.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.Nil(t, before)
	current, err := store.Current(ctx, selection.MemberID, selection.EffectiveAt)
	require.NoError(t, err)
	require.Equal(t, chosen, current)
	var catalogVersion, planID, consentText, consentVersion, explanation string
	var effectiveAt time.Time
	err = pool.QueryRow(ctx, `SELECT catalog_version, member_plan_id, consent_text, consent_version, explanation_shown, effective_at
		FROM resilience_plans WHERE resilience_plan_id = 'selection-1'`).Scan(&catalogVersion, &planID, &consentText, &consentVersion, &explanation, &effectiveAt)
	require.NoError(t, err)
	require.Equal(t, selection.CatalogVersion, catalogVersion)
	require.Equal(t, selection.MemberPlanID, planID)
	require.Equal(t, selection.ConsentText, consentText)
	require.Equal(t, selection.ConsentVersion, consentVersion)
	require.Equal(t, selection.ExplanationShown, explanation)
	require.True(t, effectiveAt.Equal(selection.EffectiveAt))
}

func TestPlanSelectionRejectsInactiveCatalog(t *testing.T) {
	pool := policyDatabase(t)
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	selection := Selection{ID: "selection-2", MemberID: "member-2", Market: "TX", CatalogVersion: "catalog-v2", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v2", ConsentText: "I accept", ConsentVersion: "consent-v1", ExplanationShown: "Reserve and reward", EffectiveAt: begin.Add(time.Hour), CorrelationID: "correlation-2"}
	_, err := store.Select(context.Background(), selection)
	require.Error(t, err)
	selection.EffectiveAt = begin.Add(48 * time.Hour)
	selected, err := store.Select(context.Background(), selection)
	require.NoError(t, err)
	require.Equal(t, "Juniper", selected.DisplayName)
	require.Equal(t, 30.0, selected.ReserveFloorPercent)
	require.Equal(t, int64(900), selected.FlexibilityRewardCents)
}

func TestCatalogMigrationReappliesAfterRollback(t *testing.T) {
	pool := policyDatabase(t)
	forward, err := os.ReadFile("../../../../../database/migrations/0008_policy_catalog.sql")
	require.NoError(t, err)
	rollback, err := os.ReadFile("../../../../../database/rollback/0008_policy_catalog.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	var linked bool
	err = pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'resilience_plans_catalog_entry_fk')`).Scan(&linked)
	require.NoError(t, err)
	require.True(t, linked)
}

func seedPolicyCatalog(t *testing.T, pool *pgxpool.Pool, begin time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO reserve_policies (policy_version, protected_hardware_floor_percent, member_plan_floor_percent, dynamic_override_percent, effective_reserve_percent, effective_at, correlation_id)
		VALUES ('policy-v1', 10, 65, 0, 65, $1, 'policy-fixture'),
		('policy-v2', 10, 30, 0, 30, $2, 'policy-fixture')`, begin.Add(-24*time.Hour), begin.Add(24*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots (catalog_version, member_plan_id, market, display_name, reserve_floor_percent, energy_plan, energy_term_months, energy_monthly_charge_cents, battery_plan, battery_term_months, battery_monthly_charge_cents, flexibility_reward_cents, effective_at, expires_at, correlation_id)
		VALUES ('catalog-v1', 'plan-cedar', 'TX', 'Cedar', 65, '{}', 0, 1999, '{}', 0, 1500, 500, $1, $2, 'catalog-fixture'),
		('catalog-v2', 'plan-cedar', 'TX', 'Juniper', 30, '{}', 0, 1799, '{}', 0, 1300, 900, $2, NULL, 'catalog-fixture')`, begin.Add(-24*time.Hour), begin.Add(24*time.Hour))
	require.NoError(t, err)
}

func policyDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	name := fmt.Sprintf("gridos_policy_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	configuration, err := pgxpool.ParseConfig(adminURL)
	require.NoError(t, err)
	configuration.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrations, err := filepath.Glob(filepath.Join(filepath.Dir(file), "../../../../../database/migrations/*.sql"))
	require.NoError(t, err)
	sort.Strings(migrations)
	for _, path := range migrations {
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		_, readErr = pool.Exec(ctx, string(contents))
		require.NoError(t, readErr, path)
	}
	return pool
}
