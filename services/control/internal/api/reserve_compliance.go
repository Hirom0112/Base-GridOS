package api

import (
	"context"
	"errors"
	"math"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	reporting "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"google.golang.org/protobuf/encoding/protojson"
)

func (source *PostgresReportSource) fillReserveCompliance(ctx context.Context, begin, end time.Time, frozen *gridosv1.OptimizationRequest, report *reporting.StoredEvent) error {
	devices := make(map[string]*gridosv1.DeviceState, len(frozen.GetDevices()))
	ids := make([]string, 0, len(frozen.GetDevices()))
	for _, device := range frozen.GetDevices() {
		if device.GetDeviceId() == "" || devices[device.GetDeviceId()] != nil {
			return errors.New("frozen reserve device identity is invalid")
		}
		capacity, reserve := device.GetUsableEnergyKwh(), device.GetEffectiveReserveKwh()
		if !finiteLiveReport(capacity) || !finiteLiveReport(reserve) || reserve < 0 || reserve > capacity {
			return errors.New("frozen reserve basis is invalid")
		}
		if capacity == 0 {
			addLiveReportGap(report, begin, end, "reserve_capacity_unavailable")
			return nil
		}
		devices[device.GetDeviceId()] = device
		ids = append(ids, device.GetDeviceId())
	}
	if len(ids) == 0 {
		addLiveReportGap(report, begin, end, "reserve_devices_unavailable")
		return nil
	}
	measured, err := source.measureReserve(ctx, ids, begin, end, devices)
	if err != nil {
		return err
	}
	if measured == nil {
		addLiveReportGap(report, begin, end, "reserve_measurement_unavailable")
		return nil
	}
	report.ReserveCompliance = measured
	return nil
}

func (source *PostgresReportSource) measureReserve(ctx context.Context, ids []string, begin, end time.Time, devices map[string]*gridosv1.DeviceState) (*reporting.ReserveCompliance, error) {
	rows, err := source.pool.Query(ctx, `SELECT device_id, payload FROM telemetry_observations
		WHERE device_id = ANY($1) AND observed_at >= $2 AND observed_at < $3
		ORDER BY observed_at, device_id, sequence`, ids, begin, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	observed := make(map[string]bool, len(ids))
	gaps := make(map[string]bool)
	touched := make(map[string]bool)
	minimum := math.Inf(1)
	for rows.Next() {
		var deviceID string
		var payload []byte
		if err := rows.Scan(&deviceID, &payload); err != nil {
			return nil, err
		}
		var observation gridosv1.TelemetryObservation
		if err := protojson.Unmarshal(payload, &observation); err != nil {
			return nil, err
		}
		if observation.GetValueState() != gridosv1.ValueState_VALUE_STATE_PRESENT {
			gaps[deviceID] = true
			continue
		}
		percent := observation.GetStateOfEnergyPercent()
		if !finiteLiveReport(percent) || percent < 0 || percent > 100 {
			return nil, errors.New("stored state of energy is invalid")
		}
		device := devices[deviceID]
		margin := percent*device.GetUsableEnergyKwh()/100 - device.GetEffectiveReserveKwh()
		minimum = math.Min(minimum, margin)
		observed[deviceID] = true
		if margin <= 1e-9 {
			touched[deviceID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(observed) == 0 {
		return nil, nil
	}
	for _, id := range ids {
		if !observed[id] {
			gaps[id] = true
		}
	}
	return &reporting.ReserveCompliance{
		DevicesExpected: uint64(len(ids)), DevicesObserved: uint64(len(observed)), MinimumMarginKWh: &minimum,
		DevicesTouchedFloor: uint64(len(touched)), ObservationGaps: uint64(len(gaps)),
		ValueKind: "MEASURED", Provenance: []string{"TELEMETRY_OBSERVATIONS", "FROZEN_EFFECTIVE_RESERVE"},
	}, nil
}
