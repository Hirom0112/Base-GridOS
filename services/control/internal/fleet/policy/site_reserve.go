package policy

import (
	"context"
	"errors"
	"time"
)

func (store *Store) SiteReserves(ctx context.Context, siteIDs []string, at time.Time) (map[string]ReserveState, error) {
	return store.SiteReservesForWindow(ctx, siteIDs, at, at)
}

func (store *Store) SiteReservesForWindow(ctx context.Context, siteIDs []string, at, through time.Time) (map[string]ReserveState, error) {
	if through.Before(at) {
		return nil, errors.New("reserve window end precedes start")
	}
	reserves := make(map[string]ReserveState)
	if len(siteIDs) == 0 {
		return reserves, nil
	}
	rows, err := store.pool.Query(ctx, `SELECT binding.site_id, plan.reserve_floor_percent,
		r.protected_hardware_floor_percent, r.dynamic_override_percent,
		COALESCE((SELECT max(o.reserve_floor_percent) FROM reserve_overrides o
			WHERE o.member_id = binding.member_id
			AND o.effective_at <= $2 AND (o.expires_at IS NULL OR o.expires_at > $2)), 0),
		(SELECT w.temporary_reserve_percent FROM travel_flex_windows w
			JOIN flexibility_offers f ON f.offer_id = w.offer_id AND f.offer_type = 'TRAVEL_FLEX' AND f.member_id = w.member_id
			WHERE w.member_id = binding.member_id AND w.policy_version = plan.policy_version
			AND plan.offer_id IS NOT NULL
			AND w.start_time <= $2 AND w.end_time > $2 AND w.end_time >= $3
			AND (w.cancelled_at IS NULL OR (w.cancelled_at > $2 AND w.cancelled_at >= $3))
			ORDER BY w.start_time DESC, w.travel_flex_window_id DESC LIMIT 1)
		FROM member_sites binding
		JOIN LATERAL (SELECT policy_version, reserve_floor_percent, offer_id FROM resilience_plans
			WHERE member_id = binding.member_id AND effective_at <= $2
			AND (expires_at IS NULL OR expires_at > $2)
			ORDER BY effective_at DESC, resilience_plan_id DESC LIMIT 1) plan ON true
		JOIN reserve_policies r ON r.policy_version = plan.policy_version
			AND r.effective_at <= $2 AND (r.expires_at IS NULL OR r.expires_at > $2)
		WHERE binding.site_id = ANY($1)`, siteIDs, at, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var siteID string
		var plan, hardware, dynamic, override float64
		var temporary *float64
		if err = rows.Scan(&siteID, &plan, &hardware, &dynamic, &override, &temporary); err != nil {
			return nil, err
		}
		base := max(plan, hardware, dynamic, override)
		state := ReserveState{BasePercent: base, EffectivePercent: base}
		if temporary != nil {
			flex := max(*temporary, hardware, dynamic, override)
			state.TravelFlexPercent = &flex
			state.EffectivePercent = flex
		}
		reserves[siteID] = state
	}
	return reserves, rows.Err()
}
