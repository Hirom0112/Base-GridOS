package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveReportUsesFrozenForecastAndStoredDelivery(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	ctx := context.Background()
	var begin, end time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT begin_time, end_time FROM dispatch_requests WHERE request_id = 'request-restart'`).Scan(&begin, &end))
	coverage := 0.9
	request := &gridosv1.OptimizationRequest{
		EventId: "event-restart", PlanVersion: 3,
		Intervals: []*gridosv1.OptimizationInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end)}},
		Sites:     []*gridosv1.ForecastSite{{SiteId: "site-1"}},
		Devices:   []*gridosv1.DeviceState{{DeviceId: "device-1"}},
		Forecast: &gridosv1.ForecastResponse{
			SiteLoads:          []*gridosv1.ForecastSiteLoad{{SiteId: "site-1", IntervalBeginTime: timestamppb.New(begin), LoadKwh: &gridosv1.ForecastValue{Value: 2, Lower: 1, Upper: 3, ModelVersion: "load-baseline-v1", FeatureVersion: "weekday-v1", ValueKind: "modeled_estimate", IntervalCoverage: &coverage}}},
			DeviceAvailability: []*gridosv1.ForecastDeviceAvailability{{DeviceId: "device-1", IntervalBeginTime: timestamppb.New(begin), Probability: &gridosv1.ForecastValue{Value: 0.8, Lower: 0.7, Upper: 0.9, ModelVersion: "availability-v1"}}},
		},
	}
	inputs, err := protojson.Marshal(request)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs)
	require.NoError(t, err)
	delivered, err := json.Marshal(report.Delivered{DeliveredMWh: 0.001, DeliveredMW: 0.001, Completeness: 1})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ('reconciliation', 'DELIVERY_VERIFIED', 'event', 'event-restart', $1, 'restart')`, delivered)
	require.NoError(t, err)
	data, err := NewPostgresReportSource(pool).EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.NotNil(t, data.Measurement)
	require.Equal(t, 0.002, data.Measurement.BaselineMWh)
	require.Equal(t, 0.002, data.Measurement.BaselineMW)
	require.Equal(t, 0.8, data.Measurement.Availability)
	require.Equal(t, "load-baseline-v1", data.Measurement.BaselineMethod)
	require.NotNil(t, data.Energy)
	require.Equal(t, 0.001, data.Energy.DeliveredMWh)
	require.Equal(t, "load-baseline-v1", data.Versions.Baseline)
	require.Equal(t, "availability-v1", data.Versions.Availability)
	require.Nil(t, data.Economics)
	require.True(t, hasLiveReportGap(data.DataGaps, "modeled_economics_unavailable"))
	require.Equal(t, 0.9, data.Measurement.Confidence)
	require.False(t, hasLiveReportGap(data.DataGaps, "baseline_confidence_unavailable"))
	request.Forecast.SiteLoads[0].LoadKwh.IntervalCoverage = nil
	inputs, err = protojson.Marshal(request)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs)
	require.NoError(t, err)
	oldData, err := NewPostgresReportSource(pool).EventReportData(ctx, "event-restart")
	require.NoError(t, err)
	require.True(t, hasLiveReportGap(oldData.DataGaps, "baseline_confidence_unavailable"))
}

func hasLiveReportGap(gaps []report.DataGap, reason string) bool {
	for _, gap := range gaps {
		if gap.Reason == reason && gap.End.After(gap.Begin) {
			return true
		}
	}
	return false
}
