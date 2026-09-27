package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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
	rows, err := store.pool.Query(ctx, `SELECT binding.site_id, binding.member_id, plan.policy_version,
		plan.reserve_floor_percent, r.protected_hardware_floor_percent, r.dynamic_override_percent,
		o.reserve_floor_percent, o.reason, o.evidence_id, o.policy_version,
		w.travel_flex_window_id, w.start_time, w.end_time, w.temporary_reserve_percent,
		w.credit_type, w.credit_cents, w.consent_version, w.policy_version
		FROM member_sites binding
		JOIN LATERAL (SELECT policy_version, reserve_floor_percent, offer_id FROM resilience_plans
			WHERE member_id = binding.member_id AND effective_at <= $2
			AND (expires_at IS NULL OR expires_at > $2)
			ORDER BY effective_at DESC, resilience_plan_id DESC LIMIT 1) plan ON true
		JOIN reserve_policies r ON r.policy_version = plan.policy_version
			AND r.effective_at <= $2 AND (r.expires_at IS NULL OR r.expires_at > $2)
		LEFT JOIN LATERAL (SELECT reserve_floor_percent, reason, COALESCE(evidence_id, reserve_override_id) AS evidence_id, policy_version FROM reserve_overrides
			WHERE member_id = binding.member_id AND effective_at <= $2
			AND (expires_at IS NULL OR expires_at > $2)
			ORDER BY reserve_floor_percent DESC, effective_at DESC, reserve_override_id DESC LIMIT 1) o ON true
		LEFT JOIN LATERAL (SELECT w.travel_flex_window_id, w.start_time, w.end_time,
			w.temporary_reserve_percent, w.credit_type, w.credit_cents, w.consent_version, w.policy_version
			FROM travel_flex_windows w JOIN flexibility_offers f
			ON f.offer_id = w.offer_id AND f.offer_type = 'TRAVEL_FLEX' AND f.member_id = w.member_id
			WHERE w.member_id = binding.member_id AND w.policy_version = plan.policy_version
			AND plan.offer_id IS NOT NULL AND w.start_time <= $2 AND w.end_time > $2 AND w.end_time >= $3
			AND (w.cancelled_at IS NULL OR (w.cancelled_at > $2 AND w.cancelled_at >= $3))
			ORDER BY w.start_time DESC, w.travel_flex_window_id DESC LIMIT 1) w ON true
		WHERE binding.site_id = ANY($1)`, siteIDs, at, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		siteID, state, err := scanSiteReserve(rows)
		if err != nil {
			return nil, err
		}
		reserves[siteID] = state
	}
	return reserves, rows.Err()
}

func scanSiteReserve(rows pgx.Rows) (string, ReserveState, error) {
	var siteID, memberID, policyVersion string
	var plan, hardware, dynamic float64
	var override, temporary *float64
	var reason, sourceID, overridePolicy, windowID, creditType, consentVersion, windowPolicy *string
	var start, end *time.Time
	var creditCents *int64
	if err := rows.Scan(&siteID, &memberID, &policyVersion, &plan, &hardware, &dynamic,
		&override, &reason, &sourceID, &overridePolicy, &windowID, &start, &end, &temporary,
		&creditType, &creditCents, &consentVersion, &windowPolicy); err != nil {
		return "", ReserveState{}, err
	}
	base := max(plan, hardware, dynamic)
	if override != nil {
		base = max(base, *override)
	}
	state := ReserveState{BasePercent: base, EffectivePercent: base, PlanPercent: plan,
		PolicyFloorPercent: max(hardware, dynamic), OverridePercent: override, PolicyVersion: policyVersion}
	if reason != nil {
		state.OverrideReason = OverrideReason(*reason)
	}
	if sourceID != nil {
		state.OverrideSourceID = *sourceID
	}
	if overridePolicy != nil {
		state.OverridePolicyVersion = *overridePolicy
	}
	if temporary != nil {
		if windowID == nil || start == nil || end == nil || creditType == nil || creditCents == nil || consentVersion == nil || windowPolicy == nil {
			return "", ReserveState{}, errors.New("consented travel flex binding is incomplete")
		}
		flex := max(*temporary, hardware, dynamic)
		if override != nil {
			flex = max(flex, *override)
		}
		state.TravelFlexPercent = &flex
		state.EffectivePercent = flex
		state.TravelFlex = &TravelFlexBinding{WindowID: *windowID, MemberID: memberID, Start: *start,
			End: *end, CreditType: CreditType(*creditType), CreditCents: *creditCents,
			ConsentVersion: *consentVersion, PolicyVersion: *windowPolicy}
	}
	return siteID, state, nil
}
