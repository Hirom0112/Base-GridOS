package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type periodicReward struct {
	memberID      string
	offerID       string
	creditType    string
	correlationID string
	zone          string
	start         time.Time
	cents         int64
	periodStart   time.Time
}

func (store *Store) PostPeriodicRewards(ctx context.Context, at time.Time) (int, error) {
	if at.IsZero() {
		return 0, errors.New("posting time required")
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rewards, err := periodicRewards(ctx, tx, at)
	if err != nil {
		return 0, err
	}
	posted := 0
	for _, reward := range rewards {
		var entryID string
		err = tx.QueryRow(ctx, `INSERT INTO reward_ledger
			(entry_id,member_id,offer_id,amount_cents,entry_type,recorded_at,correlation_id,period_start)
			VALUES (gen_random_uuid()::text,$1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (member_id,offer_id,period_start)
			WHERE event_id IS NULL AND offer_id IS NOT NULL AND period_start IS NOT NULL
			DO NOTHING RETURNING entry_id`, reward.memberID, reward.offerID, reward.cents,
			reward.creditType, at, reward.correlationID, reward.periodStart).Scan(&entryID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id,action,resource_type,resource_id,new_values,correlation_id)
			VALUES ('reward-posting','REWARD_POSTED','reward_ledger',$1,
			jsonb_build_object('member_id',$2::text,'offer_id',$3::text,'amount_cents',$4::bigint,
			'credit_type',$5::text,'period_start',$6::timestamptz),$7)`, entryID,
			reward.memberID, reward.offerID, reward.cents, reward.creditType, reward.periodStart, reward.correlationID)
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

func periodicRewards(ctx context.Context, tx pgx.Tx, at time.Time) ([]periodicReward, error) {
	rows, err := tx.Query(ctx, `SELECT flex.member_id,flex.offer_id,flex.credit_type,
		flex.correlation_id,flex.timezone,flex.start_time,offer.flexibility_reward_cents
		FROM travel_flex_windows flex JOIN flexibility_offers offer ON offer.offer_id = flex.offer_id
		AND offer.member_id = flex.member_id AND offer.offer_type = 'TRAVEL_FLEX'
		AND offer.credit_type = flex.credit_type AND offer.flexibility_reward_cents = flex.credit_cents
		WHERE flex.credit_type IN ('FIXED_DAILY','FIXED_ANNUAL')
		AND flex.start_time <= $1 AND flex.end_time > $1
		AND (flex.cancelled_at IS NULL OR flex.cancelled_at > $1)`, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rewards []periodicReward
	for rows.Next() {
		var reward periodicReward
		if err = rows.Scan(&reward.memberID, &reward.offerID, &reward.creditType,
			&reward.correlationID, &reward.zone, &reward.start, &reward.cents); err != nil {
			return nil, err
		}
		period, due, periodErr := rewardPeriod(reward, at)
		if periodErr != nil {
			return nil, periodErr
		}
		if due {
			reward.periodStart = period
			rewards = append(rewards, reward)
		}
	}
	return rewards, rows.Err()
}

func rewardPeriod(reward periodicReward, at time.Time) (time.Time, bool, error) {
	zone, err := time.LoadLocation(reward.zone)
	if err != nil {
		return time.Time{}, false, err
	}
	local := at.In(zone)
	period := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
	if reward.creditType == string(FixedDaily) {
		return period.UTC(), true, nil
	}
	start := reward.start.In(zone)
	anniversary := time.Date(local.Year(), start.Month(), start.Day(), 0, 0, 0, 0, zone)
	return period.UTC(), period.Equal(anniversary), nil
}
