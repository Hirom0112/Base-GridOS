package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLiveReportUsesFrozenForecastAndStoredDelivery(t *testing.T) {
	pool := apiTestDatabase(t)
	seedAPIEvent(t, pool)
	ctx := context.Background()
	var begin, end time.Time
	if err := pool.QueryRow(ctx, `SELECT begin_time, end_time FROM dispatch_requests WHERE request_id = 'request-restart'`).Scan(&begin, &end); err != nil {
		t.Fatal(err)
	}
	request := &gridosv1.OptimizationRequest{
		EventId: "event-restart", PlanVersion: 3,
		Intervals: []*gridosv1.OptimizationInterval{{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end)}},
		Sites:     []*gridosv1.ForecastSite{{SiteId: "site-1"}},
		Devices:   []*gridosv1.DeviceState{{DeviceId: "device-1"}},
		Forecast: &gridosv1.ForecastResponse{
			SiteLoads:          []*gridosv1.ForecastSiteLoad{{SiteId: "site-1", IntervalBeginTime: timestamppb.New(begin), LoadKwh: &gridosv1.ForecastValue{Value: 2, Lower: 1, Upper: 3, ModelVersion: "load-baseline-v1", FeatureVersion: "weekday-v1", ValueKind: "modeled_estimate"}}},
			DeviceAvailability: []*gridosv1.ForecastDeviceAvailability{{DeviceId: "device-1", IntervalBeginTime: timestamppb.New(begin), Probability: &gridosv1.ForecastValue{Value: 0.8, Lower: 0.7, Upper: 0.9, ModelVersion: "availability-v1"}}},
		},
	}
	inputs, err := protojson.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE input_snapshots SET inputs = $1 WHERE snapshot_id = 'input-restart'`, inputs); err != nil {
		t.Fatal(err)
	}
	delivered, err := json.Marshal(report.Delivered{DeliveredMWh: 0.001, DeliveredMW: 0.001, Completeness: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO audit_journal (actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ('reconciliation', 'DELIVERY_VERIFIED', 'event', 'event-restart', $1, 'restart')`, delivered); err != nil {
		t.Fatal(err)
	}
	data, err := NewPostgresReportSource(pool).EventReportData(ctx, "event-restart")
	if err != nil {
		t.Fatal(err)
	}
	if data.Measurement == nil || data.Measurement.BaselineMWh != 0.002 || data.Measurement.BaselineMW != 0.002 || data.Measurement.Availability != 0.8 || data.Measurement.BaselineMethod != "load-baseline-v1" {
		t.Fatalf("frozen baseline and availability = %+v", data.Measurement)
	}
	if data.Energy == nil || data.Energy.DeliveredMWh != 0.001 {
		t.Fatalf("stored delivery energy = %+v", data.Energy)
	}
	if data.Versions.Baseline != "load-baseline-v1" || data.Versions.Availability != "availability-v1" {
		t.Fatalf("forecast versions = %+v", data.Versions)
	}
	if data.Economics != nil || !hasLiveReportGap(data.DataGaps, "modeled_economics_unavailable") || !hasLiveReportGap(data.DataGaps, "baseline_confidence_unavailable") {
		t.Fatalf("unsourced fields = economics %+v, gaps %+v", data.Economics, data.DataGaps)
	}
}

func hasLiveReportGap(gaps []report.DataGap, reason string) bool {
	for _, gap := range gaps {
		if gap.Reason == reason && gap.End.After(gap.Begin) {
			return true
		}
	}
	return false
}
