package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func TestRewardPostingCatchesLateAcceptedReportedEventWithinDayOnce(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)
	end := begin.Add(30 * time.Minute)
	seedRewardPostingEvent(t, pool, begin, end)
	store := New(pool)
	_, err := selectWithOffer(t, store, Selection{ID: "reward-selection-one",
		MemberID: "member-one", Market: "TX", CatalogVersion: "catalog-v1",
		MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent",
		ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin.Add(-time.Minute),
		CorrelationID: "reward-event"})
	require.NoError(t, err)
	posted, err := store.PostEventRewards(ctx, "reward-event", end)
	require.NoError(t, err)
	require.Zero(t, posted)
	reportedAt := end.Add(time.Minute)
	reportBytes := []byte(`{"event_id":"reward-event","member_rewards_cents":0}`)
	digest := sha256.Sum256(reportBytes)
	_, err = pool.Exec(ctx, `INSERT INTO event_reports(event_id,version,report,sha256,produced_at)
		VALUES ('reward-event',1,$1,$2,$3)`, reportBytes, hex.EncodeToString(digest[:]), reportedAt)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE dispatch_events SET state = 'REPORTED', updated_at = $1 WHERE event_id = 'reward-event'`, reportedAt)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO command_acknowledgements
		(acknowledgement_id,command_id,idempotency_key,receipt_status,received_at,gateway_id,correlation_id)
		VALUES ('late-ack','live-command','late-ack','ACCEPTED',$1,'gateway','reward-event')`, reportedAt.Add(time.Minute))
	require.NoError(t, err)
	at := reportedAt.Add(2 * time.Minute)
	_, err = pool.Exec(ctx, `UPDATE dispatch_events SET updated_at = $1 WHERE event_id = 'reward-event'`, at.Add(-25*time.Hour))
	require.NoError(t, err)
	posted, err = store.PostLateEventRewards(ctx, at)
	require.NoError(t, err)
	require.Zero(t, posted)
	_, err = pool.Exec(ctx, `UPDATE dispatch_events SET updated_at = $1 WHERE event_id = 'reward-event'`, reportedAt)
	require.NoError(t, err)
	posted, err = store.PostLateEventRewards(ctx, at)
	require.NoError(t, err)
	require.Equal(t, 1, posted)
	posted, err = store.PostLateEventRewards(ctx, at.Add(time.Hour))
	require.NoError(t, err)
	require.Zero(t, posted)
	var rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reward_ledger WHERE event_id = 'reward-event'`).Scan(&rows))
	require.Equal(t, 1, rows)
	var storedReport []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT report FROM event_reports WHERE event_id = 'reward-event' AND version = 1`).Scan(&storedReport))
	require.Equal(t, reportBytes, storedReport)
}

func TestRewardPostingAddsFixedEventTravelCreditOnlyForFullWindow(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	begin := time.Now().UTC().Truncate(time.Second)
	end := begin.Add(30 * time.Minute)
	seedRewardPostingEvent(t, pool, begin, end)
	store := New(pool)
	for _, member := range []string{"one", "two"} {
		_, err := selectWithOffer(t, store, Selection{ID: "reward-selection-" + member,
			MemberID: "member-" + member, Market: "TX", CatalogVersion: "catalog-v1",
			MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1", ConsentText: "I consent",
			ConsentVersion: "v1", ExplanationShown: "Backup reserve", EffectiveAt: begin.Add(-2 * time.Hour),
			CorrelationID: "reward-event"})
		require.NoError(t, err)
	}
	for _, window := range []TravelFlex{
		{ID: "full-window", MemberID: "member-one", Start: begin.Add(-time.Minute), End: end.Add(time.Minute), CreditCents: 300},
		{ID: "partial-window", MemberID: "member-two", Start: begin.Add(time.Minute), End: end.Add(time.Minute), CreditCents: 400},
	} {
		window.Timezone = "UTC"
		window.TemporaryReservePercent = 20
		window.EarlyReturnAction = RestorePlanReserve
		window.CreditType = FixedEvent
		window.ConsentText = "I accept fixed event credit"
		window.ConsentVersion = "v1"
		window.PolicyVersion = "policy-v1"
		window.CorrelationID = "reward-event"
		_, err := scheduleWithOffer(t, store, window)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO command_intents
		(command_id,idempotency_key,device_id,event_id,plan_version,generation,setpoint_kw,issued_at,effective_at,
		expires_at,policy_version,correlation_id)
		VALUES ('live-two-command','live-two-command','device-two','reward-event',1,0,1,$1,$2,$3,'policy-v1','reward-event')`, begin.Add(-time.Minute), begin, end)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO command_acknowledgements
		(acknowledgement_id,command_id,idempotency_key,receipt_status,received_at,gateway_id,correlation_id)
		VALUES ('live-ack','live-command','live-ack','ACCEPTED',$1,'gateway','reward-event'),
		('live-two-ack','live-two-command','live-two-ack','ACCEPTED',$1,'gateway','reward-event')`, begin)
	require.NoError(t, err)
	posted, err := store.PostEventRewards(ctx, "reward-event", end)
	require.NoError(t, err)
	require.Equal(t, 3, posted)
	var rows int
	var total int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*),coalesce(sum(amount_cents),0)::bigint
		FROM reward_ledger WHERE event_id = 'reward-event'`).Scan(&rows, &total))
	require.Equal(t, 3, rows)
	require.Equal(t, int64(1300), total)
	var partial int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reward_ledger WHERE offer_id = 'partial-window:offer'`).Scan(&partial))
	require.Zero(t, partial)
}

func TestRewardPostingUsesLocalDailyAndAnnualPeriodsOnce(t *testing.T) {
	pool := policyDatabase(t)
	ctx := context.Background()
	zone, err := time.LoadLocation("America/Chicago")
	require.NoError(t, err)
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, zone)
	seedPolicyCatalog(t, pool, at)
	store := New(pool)
	for _, member := range []string{"daily", "annual"} {
		_, err = selectWithOffer(t, store, Selection{ID: "selection-" + member, MemberID: "member-" + member,
			Market: "TX", CatalogVersion: "catalog-v1", MemberPlanID: "plan-cedar", PolicyVersion: "policy-v1",
			ConsentText: "I consent", ConsentVersion: "v1", ExplanationShown: "Backup reserve",
			EffectiveAt: at.Add(-2 * time.Hour), CorrelationID: "periodic-test"})
		require.NoError(t, err)
	}
	for _, window := range []TravelFlex{
		{ID: "daily-window", MemberID: "member-daily", Start: at.Add(-time.Hour), End: at.AddDate(0, 0, 2), CreditType: FixedDaily, CreditCents: 300},
		{ID: "annual-window", MemberID: "member-annual", Start: at.Add(-time.Hour), End: at.AddDate(2, 0, 1), CreditType: FixedAnnual, CreditCents: 900},
	} {
		window.Timezone = zone.String()
		window.TemporaryReservePercent = 20
		window.EarlyReturnAction = RestorePlanReserve
		window.ConsentText = "I accept fixed credit"
		window.ConsentVersion = "v1"
		window.PolicyVersion = "policy-v1"
		window.CorrelationID = "periodic-test"
		_, err = scheduleWithOffer(t, store, window)
		require.NoError(t, err)
	}
	posted, err := store.PostPeriodicRewards(ctx, at.UTC())
	require.NoError(t, err)
	require.Equal(t, 2, posted)
	posted, err = store.PostPeriodicRewards(ctx, at.Add(time.Hour).UTC())
	require.NoError(t, err)
	require.Zero(t, posted)
	posted, err = store.PostPeriodicRewards(ctx, at.AddDate(0, 0, 1).UTC())
	require.NoError(t, err)
	require.Equal(t, 1, posted)
	posted, err = store.PostPeriodicRewards(ctx, at.AddDate(1, 0, 0).UTC())
	require.NoError(t, err)
	require.Equal(t, 1, posted)
	var periods int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reward_ledger
		WHERE entry_type = 'FIXED_DAILY' AND period_start = $1`, time.Date(2026, 9, 26, 0, 0, 0, 0, zone).UTC()).Scan(&periods))
	require.Equal(t, 1, periods)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reward_ledger WHERE event_id IS NULL`).Scan(&periods))
	require.Equal(t, 4, periods)
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
