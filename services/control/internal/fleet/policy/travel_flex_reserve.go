package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReserveState struct {
	BasePercent       float64
	TravelFlexPercent *float64
	EffectivePercent  float64
}

func (store *Store) EffectiveReserve(ctx context.Context, memberID string, at time.Time) (float64, error) {
	reserve, err := store.ReserveAt(ctx, memberID, at)
	return reserve.EffectivePercent, err
}

func (store *Store) ReserveAt(ctx context.Context, memberID string, at time.Time) (ReserveState, error) {
	plan, err := store.Current(ctx, memberID, at)
	if err != nil {
		return ReserveState{}, err
	}
	if plan == nil {
		return ReserveState{}, errors.New("consented effective plan is required")
	}
	var hardware, dynamic float64
	err = store.pool.QueryRow(ctx, `SELECT protected_hardware_floor_percent, dynamic_override_percent FROM reserve_policies
		WHERE policy_version = $1 AND effective_at <= $2 AND (expires_at IS NULL OR expires_at > $2)`, plan.PolicyVersion, at).Scan(&hardware, &dynamic)
	if err != nil {
		return ReserveState{}, err
	}
	var override *float64
	err = store.pool.QueryRow(ctx, `SELECT max(reserve_floor_percent) FROM reserve_overrides
		WHERE member_id = $1 AND effective_at <= $2 AND (expires_at IS NULL OR expires_at > $2)`, memberID, at).Scan(&override)
	if err != nil {
		return ReserveState{}, err
	}
	base := max(plan.ReserveFloorPercent, hardware, dynamic)
	if override != nil {
		base = max(base, *override)
	}
	state := ReserveState{BasePercent: base, EffectivePercent: base}
	var temporary float64
	var policyVersion string
	err = store.pool.QueryRow(ctx, `SELECT temporary_reserve_percent, policy_version FROM travel_flex_windows
		WHERE member_id = $1 AND start_time <= $2 AND end_time > $2
		AND (cancelled_at IS NULL OR cancelled_at > $2)
		ORDER BY start_time DESC, travel_flex_window_id DESC LIMIT 1`, memberID, at).Scan(&temporary, &policyVersion)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ReserveState{}, err
	}
	if err == nil && policyVersion == plan.PolicyVersion {
		flex := max(temporary, hardware, dynamic)
		if override != nil {
			flex = max(flex, *override)
		}
		state.TravelFlexPercent = &flex
		state.EffectivePercent = flex
	}
	return state, nil
}
