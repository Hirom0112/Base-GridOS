package policy

import (
	"context"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
)

func (bridge *RiskBridge) EvaluateAnomalies(ctx context.Context, at time.Time) error {
	deviceIDs := make([]string, 0, len(bridge.sites))
	for _, site := range bridge.sites {
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
