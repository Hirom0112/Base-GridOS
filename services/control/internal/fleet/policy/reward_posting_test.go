package policy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestRewardPostingRequiresAcceptedNonzeroCommandAndPaysExpiredConsent(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Now().UTC().Truncate(time.Second)
	end := begin.Add(30 * time.Minute)
	seedRewardPostingEvent(t, pool, begin, end)
	for _, member := range []string{"one", "two"} {
		_, err := selectWithOffer(t, New(pool), Selection{ID: "reward-selection-" + member,
			MemberID: "member-" + member, Market: "TX", CatalogVersion: "catalog-v1",
			MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent",
			ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin.Add(-2 * time.Hour),
			CorrelationID: "reward-event"})
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO command_acknowledgements
		(acknowledgement_id,command_id,idempotency_key,receipt_status,received_at,gateway_id,correlation_id)
		VALUES ('zero-ack','zero-command','zero-ack','ACCEPTED',$1,'gateway','reward-event')`, begin)
	require.NoError(t, err)
	posted, err := New(pool).PostEventRewards(ctx, "reward-event", end)
	require.NoError(t, err)
	require.Zero(t, posted)
	_, err = pool.Exec(ctx, `INSERT INTO command_acknowledgements
		(acknowledgement_id,command_id,idempotency_key,receipt_status,received_at,gateway_id,correlation_id)
		VALUES ('live-ack','live-command','live-ack','ACCEPTED',$1,'gateway','reward-event')`, begin)
	require.NoError(t, err)
	posted, err = New(pool).PostEventRewards(ctx, "reward-event", end)
	require.NoError(t, err)
	require.Equal(t, 1, posted)
	posted, err = New(pool).PostEventRewards(ctx, "reward-event", end.Add(time.Minute))
	require.NoError(t, err)
	require.Zero(t, posted)
	var member string
	var amount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_id,amount_cents FROM reward_ledger WHERE event_id = 'reward-event'`).Scan(&member, &amount))
	require.Equal(t, "member-one", member)
	require.Equal(t, int64(500), amount)
}

func seedRewardPostingEvent(t *testing.T, pool *pgxpool.Pool, begin, end time.Time) {
	t.Helper()
	ctx := context.Background()
	seedPolicyCatalog(t, pool, begin)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id,member_id,bound_at,source,provenance)
		VALUES ('site-one','member-one',$1,'SIMULATED','{"provenance":"SIMULATED"}'),
		('site-two','member-two',$1,'SIMULATED','{"provenance":"SIMULATED"}')`, begin.Add(-3*time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO dispatch_requests
		(request_id,event_type,begin_time,end_time,target_kw,measurement_boundary,load_zones,correlation_id)
		VALUES ('reward-request','GRID_SERVICE',$1,$2,1,'SITE',ARRAY['TX'],'reward-event')`, begin, end)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO dispatch_events(event_id,request_id,state,plan_version,correlation_id)
		VALUES ('reward-event','reward-request','RECONCILED',1,'reward-event')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO input_snapshots(snapshot_id,event_id,captured_at,inputs,provenance,correlation_id)
		VALUES ('reward-input','reward-event',$1,
		'{"devices":[{"deviceId":"device-one","siteId":"site-one"},{"deviceId":"device-two","siteId":"site-two"}]}',
		'{"source":"SIMULATED"}','reward-event')`, begin.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO eligibility_snapshots
		(snapshot_id,event_id,captured_at,eligible_device_ids,exclusions,policy_version,correlation_id)
		VALUES ('reward-eligibility','reward-event',$1,ARRAY['device-one','device-two'],'{}','policy-v1','reward-event')`, begin.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO plan_versions(event_id,version,input_snapshot_id,eligibility_snapshot_id,plan,solver_version,model_version,correlation_id)
		VALUES ('reward-event',1,'reward-input','reward-eligibility','{}','solver','model','reward-event')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO command_intents
		(command_id,idempotency_key,device_id,event_id,plan_version,generation,setpoint_kw,issued_at,effective_at,
		expires_at,policy_version,correlation_id)
		VALUES ('live-command','live-command','device-one','reward-event',1,0,1,$1,$2,$3,'policy-v1','reward-event'),
		('zero-command','zero-command','device-two','reward-event',1,0,0,$1,$2,$3,'policy-v1','reward-event')`, begin.Add(-time.Minute), begin, end)
	require.NoError(t, err)
}
