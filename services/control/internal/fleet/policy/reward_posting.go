package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type eventReward struct {
	memberID string
	offerID  string
	cents    int64
}

func (store *Store) PostEventRewards(ctx context.Context, eventID string, at time.Time) (int, error) {
	if eventID == "" || at.IsZero() {
		return 0, errors.New("event and posting time required")
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var begin, end time.Time
	var state, correlationID string
	err = tx.QueryRow(ctx, `SELECT request.begin_time,request.end_time,event.state,event.correlation_id
		FROM dispatch_events event JOIN dispatch_requests request USING (request_id)
		WHERE event.event_id = $1 FOR UPDATE OF event`, eventID).Scan(&begin, &end, &state, &correlationID)
	if err != nil {
		return 0, err
	}
	if state != "RECONCILED" && state != "REPORTED" {
		return 0, errors.New("event is not ready for reward posting")
	}
	rewards, err := eventRewards(ctx, tx, eventID, begin, end)
	if err != nil {
		return 0, err
	}
	posted := 0
	for _, reward := range rewards {
		var entryID string
		err = tx.QueryRow(ctx, `INSERT INTO reward_ledger
			(entry_id,member_id,event_id,offer_id,amount_cents,entry_type,recorded_at,correlation_id)
			VALUES (gen_random_uuid()::text,$1,$2,$3,$4,'FIXED_EVENT',$5,$6)
			ON CONFLICT (event_id,member_id,offer_id) WHERE event_id IS NOT NULL AND offer_id IS NOT NULL
			DO NOTHING RETURNING entry_id`, reward.memberID, eventID, reward.offerID, reward.cents, at, correlationID).Scan(&entryID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id,action,resource_type,resource_id,new_values,correlation_id)
			VALUES ('reward-posting','REWARD_POSTED','reward_ledger',$1,
			jsonb_build_object('event_id',$2::text,'member_id',$3::text,'offer_id',$4::text,'amount_cents',$5::bigint),$6)`,
			entryID, eventID, reward.memberID, reward.offerID, reward.cents, correlationID)
		if err != nil {
			return 0, err
		}
		posted++
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return posted, nil
}

func eventRewards(ctx context.Context, tx pgx.Tx, eventID string, begin, end time.Time) ([]eventReward, error) {
	rows, err := tx.Query(ctx, `WITH participants AS (
		SELECT DISTINCT site.member_id FROM command_intents command
		JOIN command_acknowledgements acknowledgement ON acknowledgement.command_id = command.command_id
			AND acknowledgement.receipt_status = 'ACCEPTED'
		JOIN plan_versions plan ON plan.event_id = command.event_id AND plan.version = command.plan_version
		JOIN input_snapshots snapshot ON snapshot.snapshot_id = plan.input_snapshot_id
		JOIN LATERAL jsonb_array_elements(snapshot.inputs->'devices') device
			ON device->>'deviceId' = command.device_id
		JOIN member_sites site ON site.site_id = device->>'siteId' AND site.bound_at <= $2
		WHERE command.event_id = $1 AND command.setpoint_kw <> 0
	)
	SELECT participant.member_id,offer.offer_id,offer.flexibility_reward_cents
	FROM participants participant
	JOIN LATERAL (SELECT offer_id FROM resilience_plans plan
		WHERE plan.member_id = participant.member_id AND plan.effective_at <= $2
		AND (plan.expires_at IS NULL OR plan.expires_at > $2)
		ORDER BY plan.effective_at DESC,plan.resilience_plan_id DESC LIMIT 1) selected ON true
	JOIN flexibility_offers offer ON offer.offer_id = selected.offer_id AND offer.member_id = participant.member_id
		AND offer.offer_type = 'PLAN'
	UNION ALL
	SELECT participant.member_id,offer.offer_id,offer.flexibility_reward_cents
	FROM participants participant
	JOIN travel_flex_windows flex ON flex.member_id = participant.member_id AND flex.start_time <= $2
		AND flex.end_time >= $3 AND (flex.cancelled_at IS NULL OR flex.cancelled_at >= $3)
		AND flex.credit_type = 'FIXED_EVENT'
	JOIN flexibility_offers offer ON offer.offer_id = flex.offer_id AND offer.member_id = participant.member_id
		AND offer.offer_type = 'TRAVEL_FLEX'`, eventID, begin, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []eventReward
	for rows.Next() {
		var reward eventReward
		if err = rows.Scan(&reward.memberID, &reward.offerID, &reward.cents); err != nil {
			return nil, err
		}
		rewards = append(rewards, reward)
	}
	return rewards, rows.Err()
}
