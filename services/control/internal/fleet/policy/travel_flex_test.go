package policy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTravelFlexAppliesOnlyInsideConsentedLocalWindow(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	zone, err := time.LoadLocation("America/Chicago")
	require.NoError(t, err)
	begin := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err = store.Select(ctx, Selection{
		ID: "selection-flex", MemberID: "member-flex", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1",
		ConsentText: "I accept Cedar", ConsentVersion: "consent-v1", ExplanationShown: "Backup reserve and reward",
		EffectiveAt: begin.Add(time.Hour), CorrelationID: "selection-flex",
	})
	require.NoError(t, err)
	start := time.Date(2026, 11, 1, 9, 0, 0, 0, zone)
	end := time.Date(2026, 11, 5, 17, 0, 0, 0, zone)
	window := TravelFlex{
		ID: "flex-1", MemberID: "member-flex", Start: start, End: end, Timezone: "America/Chicago",
		TemporaryReservePercent: 20, EarlyReturnAction: "RESTORE_PLAN_RESERVE", CreditType: "FIXED_DAILY", CreditCents: 750,
		ConsentText: "I accept a fixed daily credit", ConsentVersion: "flex-consent-v1", PolicyVersion: "policy-v1", CorrelationID: "flex-correlation",
	}
	_, err = store.ScheduleTravelFlex(ctx, TravelFlex{ID: "without-consent", MemberID: window.MemberID, Start: start, End: end, Timezone: window.Timezone, TemporaryReservePercent: 20, EarlyReturnAction: window.EarlyReturnAction, CreditType: window.CreditType, CreditCents: window.CreditCents, PolicyVersion: window.PolicyVersion, CorrelationID: window.CorrelationID})
	require.Error(t, err)
	before, err := store.EffectiveReserve(ctx, window.MemberID, start.Add(-time.Nanosecond))
	require.NoError(t, err)
	require.Equal(t, 65.0, before)
	stored, err := store.ScheduleTravelFlex(ctx, window)
	require.NoError(t, err)
	require.Equal(t, CreditType("FIXED_DAILY"), stored.CreditType)
	require.Equal(t, int64(750), stored.CreditCents)
	require.Equal(t, "America/Chicago", stored.Timezone)
	inside, err := store.EffectiveReserve(ctx, window.MemberID, start)
	require.NoError(t, err)
	require.Equal(t, 20.0, inside)
	after, err := store.EffectiveReserve(ctx, window.MemberID, end)
	require.NoError(t, err)
	require.Equal(t, 65.0, after)
	var consent, version, timezone, creditType string
	var credit int64
	err = pool.QueryRow(ctx, `SELECT consent_text, consent_version, timezone, credit_type, credit_cents
		FROM travel_flex_windows WHERE travel_flex_window_id = 'flex-1'`).Scan(&consent, &version, &timezone, &creditType, &credit)
	require.NoError(t, err)
	require.Equal(t, window.ConsentText, consent)
	require.Equal(t, window.ConsentVersion, version)
	require.Equal(t, window.Timezone, timezone)
	require.Equal(t, string(window.CreditType), creditType)
	require.Equal(t, window.CreditCents, credit)
}

func TestTravelFlexEarlyReturnRestoresPlanReserveOnce(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err := store.Select(ctx, Selection{ID: "selection-return", MemberID: "member-return", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I accept Cedar", ConsentVersion: "consent-v1", ExplanationShown: "Backup reserve", EffectiveAt: begin, CorrelationID: "selection-return"})
	require.NoError(t, err)
	start := begin.Add(time.Hour)
	window := TravelFlex{ID: "flex-return", MemberID: "member-return", Start: start, End: start.Add(72 * time.Hour), Timezone: "UTC", TemporaryReservePercent: 20, EarlyReturnAction: "RESTORE_PLAN_RESERVE", CreditType: "FIXED_EVENT", CreditCents: 1200, ConsentText: "I accept fixed event credit", ConsentVersion: "flex-consent-v1", PolicyVersion: "policy-v1", CorrelationID: "flex-return"}
	_, err = store.ScheduleTravelFlex(ctx, window)
	require.NoError(t, err)
	returnedAt := start.Add(24 * time.Hour)
	command := EarlyReturn{ID: "return-1", WindowID: window.ID, MemberID: window.MemberID, At: returnedAt, CorrelationID: "return-correlation"}
	require.NoError(t, store.EndTravelFlexEarly(ctx, command))
	require.NoError(t, store.EndTravelFlexEarly(ctx, command))
	command.ID = "return-2"
	require.Error(t, store.EndTravelFlexEarly(ctx, command))
	reserve, err := store.EffectiveReserve(ctx, window.MemberID, returnedAt)
	require.NoError(t, err)
	require.Equal(t, 65.0, reserve)
	var cancellations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE action = 'TRAVEL_FLEX_CANCELLED' AND resource_id = 'flex-return'`).Scan(&cancellations))
	require.Equal(t, 1, cancellations)
}

func TestTravelFlexMaximumReturnUsesCatalogBandUntilWindowEnd(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	_, err := pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots (catalog_version, member_plan_id, market, display_name, reserve_floor_percent, energy_plan, energy_term_months, energy_monthly_charge_cents, battery_plan, battery_term_months, battery_monthly_charge_cents, flexibility_reward_cents, effective_at, expires_at, correlation_id)
		VALUES ('catalog-v1', 'plan-fortress', 'TX', 'Fortress', 80, '{}', 0, 2200, '{}', 0, 1600, 250, $1, $2, 'catalog-fixture')`, begin.Add(-24*time.Hour), begin.Add(24*time.Hour))
	require.NoError(t, err)
	store := New(pool)
	_, err = store.Select(ctx, Selection{ID: "selection-max", MemberID: "member-max", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I accept Cedar", ConsentVersion: "consent-v1", ExplanationShown: "Backup reserve", EffectiveAt: begin, CorrelationID: "selection-max"})
	require.NoError(t, err)
	start := begin.Add(time.Hour)
	end := start.Add(8 * time.Hour)
	_, err = store.ScheduleTravelFlex(ctx, TravelFlex{ID: "flex-max", MemberID: "member-max", Start: start, End: end, Timezone: "UTC", TemporaryReservePercent: 20, EarlyReturnAction: "RESTORE_MAXIMUM_RESERVE", CreditType: "FIXED_EVENT", CreditCents: 1200, ConsentText: "I accept fixed event credit", ConsentVersion: "flex-consent-v1", PolicyVersion: "policy-v1", CorrelationID: "flex-max"})
	require.NoError(t, err)
	returnedAt := start.Add(time.Hour)
	require.NoError(t, store.EndTravelFlexEarly(ctx, EarlyReturn{ID: "return-max", WindowID: "flex-max", MemberID: "member-max", At: returnedAt, CorrelationID: "return-max"}))
	reserve, err := store.EffectiveReserve(ctx, "member-max", returnedAt)
	require.NoError(t, err)
	require.Equal(t, 80.0, reserve)
	reserve, err = store.EffectiveReserve(ctx, "member-max", end)
	require.NoError(t, err)
	require.Equal(t, 65.0, reserve)
	var floor float64
	var until time.Time
	err = pool.QueryRow(ctx, `SELECT reserve_floor_percent, expires_at FROM reserve_overrides WHERE member_id = 'member-max' AND reason = 'EARLY_RETURN'`).Scan(&floor, &until)
	require.NoError(t, err)
	require.Equal(t, 80.0, floor)
	require.True(t, until.Equal(end))
}

func TestTravelFlexMigrationReappliesAfterRollback(t *testing.T) {
	pool := policyDatabase(t)
	forward, err := os.ReadFile("../../../../../database/migrations/0009_travel_flex_return.sql")
	require.NoError(t, err)
	rollback, err := os.ReadFile("../../../../../database/rollback/0009_travel_flex_return.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	var present bool
	err = pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'travel_flex_windows'::regclass AND attname = 'end_idempotency_key' AND NOT attisdropped)`).Scan(&present)
	require.NoError(t, err)
	require.True(t, present)
}
