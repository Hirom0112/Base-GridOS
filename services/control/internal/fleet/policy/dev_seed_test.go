package policy

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRewardPostingSeedFreezesFixedEventCredit(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	seed, err := os.ReadFile("../../../../../database/seeds/dev.sql")
	require.NoError(t, err)
	begin := strings.Index(string(seed), "INSERT INTO reserve_policies (")
	end := strings.LastIndex(string(seed), "COMMIT;")
	require.GreaterOrEqual(t, begin, 0)
	require.Greater(t, end, begin)
	_, err = pool.Exec(ctx, string(seed[begin:end]))
	require.NoError(t, err)
	for planID, cents := range map[string]int64{"essential": 500, "balanced": 800, "maximum": 1200} {
		var stored int64
		var provenance string
		require.NoError(t, pool.QueryRow(ctx, `SELECT flexibility_reward_cents, provenance->>'provenance'
			FROM pricing_catalog_snapshots WHERE catalog_version = 'catalog-sim-1' AND member_plan_id = $1`, planID).Scan(&stored, &provenance))
		require.Equal(t, cents, stored)
		require.Equal(t, "SIMULATED", provenance)
	}
	now := time.Now().UTC()
	offer, err := New(pool).PresentOffer(ctx, Offer{ID: "seed-offer", MemberID: "seed-member", Kind: PlanOffer,
		Market: "ERCOT", CatalogVersion: "catalog-sim-1", MemberPlanID: "essential", ContractVersion: "sim-1",
		PriceText: "SIMULATED", ConsentText: "I accept Essential", ConsentVersion: "sim-1",
		EffectiveAt: now, ExpiresAt: now.Add(time.Hour), CorrelationID: "seed-offer"})
	require.NoError(t, err)
	require.Equal(t, int64(500), offer.FlexibilityRewardCents)
}
