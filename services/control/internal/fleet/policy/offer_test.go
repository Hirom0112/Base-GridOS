package policy

import (
	"context"
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
