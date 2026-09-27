package policy

import (
	"context"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

func (bridge *RiskBridge) EvaluateAnomalies(ctx context.Context, at time.Time) error {
	eligible, err := bridge.activeAnomalySites(ctx, at)
	if err != nil {
		return err
	}
	deviceIDs := make([]string, 0, len(bridge.sites))
	for _, site := range bridge.sites {
		if !eligible[site.GetSite().GetSiteId()] {
			continue
		}
		for _, device := range site.GetDevices() {
			deviceIDs = append(deviceIDs, device.GetDeviceId())
		}
	}
	latest, err := storage.NewTelemetryStore(bridge.pool).Latest(ctx, deviceIDs, at.AddDate(0, 0, -7))
	if err != nil {
		return err
	}
	byDevice := make(map[string]*gridosv1.TelemetryObservation, len(latest))
	for _, observation := range latest {
		byDevice[observation.GetDeviceId()] = observation
	}
	for _, site := range bridge.sites {
		if !eligible[site.GetSite().GetSiteId()] {
			continue
		}
		var newest *gridosv1.TelemetryObservation
		for _, device := range site.GetDevices() {
			observation := byDevice[device.GetDeviceId()]
			if observation == nil || observation.GetPowerFlow() == nil || observation.GetValueState() != gridosv1.ValueState_VALUE_STATE_PRESENT || observation.GetObservationTime().AsTime().After(at) {
				continue
			}
			if newest == nil || observation.GetObservationTime().AsTime().After(newest.GetObservationTime().AsTime()) {
				newest = observation
			}
		}
		if newest == nil {
			continue
		}
		_, err = bridge.store.EvaluateAnomaly(ctx, MeasuredSiteLoad{ObservationID: newest.GetObservationId(),
			SiteID: site.GetSite().GetSiteId(), ObservedAt: newest.GetObservationTime().AsTime(),
			ToHomeKW: newest.GetPowerFlow().GetToHomeKw(), CorrelationID: newest.GetObservationId()})
		if err != nil {
			return err
		}
	}
	return nil
}

func (bridge *RiskBridge) activeAnomalySites(ctx context.Context, at time.Time) (map[string]bool, error) {
	rows, err := bridge.pool.Query(ctx, `SELECT site.site_id FROM member_sites site
		JOIN LATERAL (SELECT opted_in FROM member_anomaly_preferences
			WHERE member_id = site.member_id AND effective_at <= $1 AND expires_at > $1
			ORDER BY effective_at DESC, preference_id DESC LIMIT 1) preference ON preference.opted_in
		WHERE EXISTS (SELECT 1 FROM member_away_periods away
			WHERE away.member_id = site.member_id AND away.start_time <= $1 AND away.end_time > $1
			AND (away.ended_at IS NULL OR away.ended_at > $1))
		OR EXISTS (SELECT 1 FROM travel_flex_windows flex JOIN flexibility_offers offer
			ON offer.offer_id = flex.offer_id AND offer.offer_type = 'TRAVEL_FLEX' AND offer.member_id = flex.member_id
			WHERE flex.member_id = site.member_id AND flex.start_time <= $1 AND flex.end_time > $1
			AND (flex.cancelled_at IS NULL OR flex.cancelled_at > $1))`, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	eligible := make(map[string]bool)
	for rows.Next() {
		var siteID string
		if err = rows.Scan(&siteID); err != nil {
			return nil, err
		}
		eligible[siteID] = true
	}
	return eligible, rows.Err()
}
