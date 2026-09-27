package policy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOfferPersistsExactPresentedPlanTermsOnce(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	offer := Offer{ID: "offer-plan", MemberID: "member-offer", Kind: "PLAN", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", ContractVersion: "contract-v1", PriceText: "Energy $19.99 and battery $15.00 monthly; reward $5.00", ConsentText: "I accept Cedar terms", ConsentVersion: "consent-v1", EffectiveAt: begin, ExpiresAt: begin.Add(time.Hour), CorrelationID: "offer-plan"}
	presented, err := store.PresentOffer(ctx, offer)
	require.NoError(t, err)
	require.Equal(t, int64(1999), presented.EnergyMonthlyChargeCents)
	require.Equal(t, int64(1500), presented.BatteryMonthlyChargeCents)
	require.Equal(t, int64(500), presented.FlexibilityRewardCents)
	retried, err := store.PresentOffer(ctx, offer)
	require.NoError(t, err)
	require.True(t, sameOffer(*presented, *retried))
	offer.PriceText = "Changed price"
	_, err = store.PresentOffer(ctx, offer)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE flexibility_offers SET price_text = 'Changed price' WHERE offer_id = 'offer-plan'`)
	require.Error(t, err)
	var price, consent, version string
	require.NoError(t, pool.QueryRow(ctx, `SELECT price_text, consent_text, consent_version FROM flexibility_offers WHERE offer_id = 'offer-plan'`).Scan(&price, &consent, &version))
	require.Equal(t, "Energy $19.99 and battery $15.00 monthly; reward $5.00", price)
	require.Equal(t, "I accept Cedar terms", consent)
	require.Equal(t, "consent-v1", version)
}

func TestOfferSelectionRequiresExactPresentedTerms(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	offer := Offer{ID: "offer-selection", MemberID: "member-selection", Kind: PlanOffer, Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", ContractVersion: "contract-v1", PriceText: "Energy $19.99 and battery $15.00 monthly; reward $5.00", ConsentText: "I accept Cedar terms", ConsentVersion: "consent-v1", EffectiveAt: begin, ExpiresAt: begin.Add(time.Hour), CorrelationID: "offer-selection"}
	_, err := store.PresentOffer(ctx, offer)
	require.NoError(t, err)
	selection := Selection{ID: "selection-offer", MemberID: offer.MemberID, Market: offer.Market, CatalogVersion: offer.CatalogVersion, MemberPlanID: offer.MemberPlanID, PolicyVersion: "policy-v1", ConsentText: offer.ConsentText, ConsentVersion: offer.ConsentVersion, ExplanationShown: "Backup reserve", EffectiveAt: begin, CorrelationID: "selection-offer"}
	_, err = store.Select(ctx, selection)
	require.Error(t, err)
	selection.OfferID = offer.ID
	selection.ConsentText = "Changed consent"
	_, err = store.Select(ctx, selection)
	require.Error(t, err)
	selection.ConsentText = offer.ConsentText
	selected, err := store.Select(ctx, selection)
	require.NoError(t, err)
	require.Equal(t, offer.ID, selected.OfferID)
	var storedOffer string
	require.NoError(t, pool.QueryRow(ctx, `SELECT offer_id FROM resilience_plans WHERE resilience_plan_id = $1`, selection.ID).Scan(&storedOffer))
	require.Equal(t, offer.ID, storedOffer)
}

func TestOfferTravelFlexRequiresExactFixedCreditAndConsent(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err := selectWithOffer(t, store, Selection{ID: "selection-flex-offer", MemberID: "member-flex-offer", Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I accept Cedar", ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin, CorrelationID: "selection-flex-offer"})
	require.NoError(t, err)
	reserve := 20.0
	start := begin.Add(time.Minute)
	end := start.Add(time.Hour)
	offer := Offer{ID: "offer-flex", MemberID: "member-flex-offer", Kind: TravelFlexOffer, Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", ContractVersion: "flex-contract-v1", PriceText: "Fixed $12 event credit", ConsentText: "I accept fixed credit", ConsentVersion: "flex-consent-v1", EffectiveAt: start, ExpiresAt: end, TemporaryReservePercent: &reserve, CreditType: FixedEvent, CreditCents: 1200, CorrelationID: "offer-flex"}
	presented, err := store.PresentOffer(ctx, offer)
	require.NoError(t, err)
	require.Equal(t, int64(1200), presented.FlexibilityRewardCents)
	window := TravelFlex{ID: "window-offer", MemberID: offer.MemberID, Start: start, End: end, Timezone: "UTC", TemporaryReservePercent: reserve, EarlyReturnAction: RestorePlanReserve, CreditType: FixedEvent, CreditCents: 1200, ConsentText: offer.ConsentText, ConsentVersion: offer.ConsentVersion, PolicyVersion: "policy-v1", CorrelationID: "window-offer"}
	_, err = store.ScheduleTravelFlex(ctx, window)
	require.Error(t, err)
	window.OfferID = offer.ID
	window.CreditCents = 1300
	_, err = store.ScheduleTravelFlex(ctx, window)
	require.Error(t, err)
	window.CreditCents = 1200
	stored, err := store.ScheduleTravelFlex(ctx, window)
	require.NoError(t, err)
	require.Equal(t, offer.ID, stored.OfferID)
	var storedOffer string
	require.NoError(t, pool.QueryRow(ctx, `SELECT offer_id FROM travel_flex_windows WHERE travel_flex_window_id = $1`, window.ID).Scan(&storedOffer))
	require.Equal(t, offer.ID, storedOffer)
}

func TestOfferLegacyUnknownTermsNeverUnlockTravelFlex(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	_, err := pool.Exec(ctx, `INSERT INTO resilience_plans(resilience_plan_id, member_id, market, reserve_floor_percent, consent_text, consent_version, policy_version, effective_at, correlation_id, catalog_version, member_plan_id, explanation_shown)
		VALUES ('legacy-plan', 'member-legacy', 'TX', 65, 'Legacy consent', 'v1', 'policy-v1', $1, 'legacy', 'catalog-v1', 'plan-cedar', 'Legacy explanation')`, begin)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO travel_flex_windows(travel_flex_window_id, member_id, start_time, end_time, timezone, temporary_reserve_percent, early_return_action, credit_type, credit_cents, consent_text, consent_version, policy_version, correlation_id)
		VALUES ('legacy-window', 'member-legacy', $1, $2, 'UTC', 20, 'RESTORE_PLAN_RESERVE', 'FIXED_EVENT', 100, 'Legacy consent', 'v1', 'policy-v1', 'legacy')`, begin, begin.Add(time.Hour))
	require.NoError(t, err)
	reserve, err := New(pool).ReserveAt(ctx, "member-legacy", begin)
	require.NoError(t, err)
	require.Nil(t, reserve.TravelFlexPercent)
	require.Equal(t, 65.0, reserve.EffectivePercent)
}

func TestLedgerRequiresOfferAndIsAppendOnly(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	seedPolicyCatalog(t, pool, begin)
	store := New(pool)
	_, err := store.PresentOffer(ctx, Offer{ID: "offer-ledger", MemberID: "member-ledger", Kind: PlanOffer, Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", ContractVersion: "contract-v1", PriceText: "Reward $5.00", ConsentText: "I accept", ConsentVersion: "v1", EffectiveAt: begin, ExpiresAt: begin.Add(time.Hour), CorrelationID: "offer-ledger"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO reward_ledger(entry_id, member_id, offer_id, amount_cents, entry_type, recorded_at, correlation_id)
		VALUES ('entry-without-offer', 'member-ledger', NULL, 500, 'EARNED', $1, 'ledger')`, begin)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO reward_ledger(entry_id, member_id, offer_id, amount_cents, entry_type, recorded_at, correlation_id)
		VALUES ('entry-ledger', 'member-ledger', 'offer-ledger', 500, 'EARNED', $1, 'ledger')`, begin)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE reward_ledger SET amount_cents = 600 WHERE entry_id = 'entry-ledger'`)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM reward_ledger WHERE entry_id = 'entry-ledger'`)
	require.Error(t, err)
	var catalog, contract, consent string
	require.NoError(t, pool.QueryRow(ctx, `SELECT offer.catalog_version, offer.contract_version, offer.consent_version
		FROM reward_ledger ledger JOIN flexibility_offers offer ON offer.offer_id = ledger.offer_id
		WHERE ledger.entry_id = 'entry-ledger'`).Scan(&catalog, &contract, &consent))
	require.Equal(t, "catalog-v1", catalog)
	require.Equal(t, "contract-v1", contract)
	require.Equal(t, "v1", consent)
}

func TestOfferMigrationReappliesAfterRollback(t *testing.T) {
	pool := policyDatabase(t)
	forward, err := os.ReadFile("../../../../../database/migrations/0012_presented_offers.sql")
	require.NoError(t, err)
	rollback, err := os.ReadFile("../../../../../database/rollback/0012_presented_offers.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(forward))
	require.NoError(t, err)
	var present bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'flexibility_offers_append_only')`).Scan(&present))
	require.True(t, present)
}
