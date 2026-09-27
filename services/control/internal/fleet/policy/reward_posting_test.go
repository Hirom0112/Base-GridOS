package policy

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRewardPostingMigrationReapplies(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	var present bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('reward_ledger') IS NOT NULL`).Scan(&present))
	require.True(t, present)
	forward, err := os.ReadFile("../../../../../database/migrations/0016_reward_posting.sql")
	require.NoError(t, err)
	rollback, err := os.ReadFile("../../../../../database/rollback/0016_reward_posting.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(forward))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(rollback))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(forward))
	require.NoError(t, err)
	var indexes int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE tablename = 'reward_ledger'
		AND indexname IN ('reward_ledger_event_member_offer_idx', 'reward_ledger_member_offer_period_idx')`).Scan(&indexes))
	require.Equal(t, 2, indexes)
}
