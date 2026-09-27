package report

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	core "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/stretchr/testify/require"
)

type reportSource struct {
	published     *core.EventReport
	publishedByID map[string]*core.EventReport
	versions      map[string]map[uint64]*core.EventReport
	live          core.StoredEvent
	liveReads     int
}

func (source *reportSource) StoredReport(_ context.Context, eventID string) (*core.EventReport, error) {
	if source.publishedByID != nil {
		return source.publishedByID[eventID], nil
	}
	return source.published, nil
}

func (source *reportSource) StoredReportVersion(_ context.Context, eventID string, version uint64) (*core.EventReport, error) {
	return source.versions[eventID][version], nil
}

func TestCompareEventReportsRequiresRoleAndReturnsDifferences(t *testing.T) {
	rewardsA, rewardsB := int64(100), int64(250)
	source := &reportSource{publishedByID: map[string]*core.EventReport{
		"event-a": {EventID: "event-a", PlanVersion: 2, RequestedMW: 1, MemberRewardsCents: &rewardsA},
		"event-b": {EventID: "event-b", PlanVersion: 3, RequestedMW: 2, MemberRewardsCents: &rewardsB},
	}, versions: map[string]map[uint64]*core.EventReport{
		"event-a": {1: {EventID: "event-a", PlanVersion: 1, RequestedMW: 0.5, MemberRewardsCents: &rewardsA}},
	}}
	_, handler := gridosv1connect.NewReportServiceHandler(NewService(source))
	server := httptest.NewServer(handler)
	defer server.Close()
	client := gridosv1connect.NewReportServiceClient(http.DefaultClient, server.URL)
	request := connect.NewRequest(&gridosv1.CompareEventReportsRequest{EventIdA: "event-a", EventIdB: "event-b"})
	_, err := client.CompareEventReports(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	request.Header().Set("X-GridOS-Role", "partner")
	_, err = client.CompareEventReports(context.Background(), request)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	request.Header().Set("X-GridOS-Role", "analyst")
	response, err := client.CompareEventReports(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, 3, len(response.Msg.GetDifferences()))
	require.Equal(t, "member_rewards_cents", response.Msg.GetDifferences()[0].GetField())
	require.Equal(t, "100", response.Msg.GetDifferences()[0].GetBefore())
	require.Equal(t, "250", response.Msg.GetDifferences()[0].GetAfter())
	require.Equal(t, 0, source.liveReads)
	version := uint64(1)
	request.Msg.PlanVersionA = &version
	response, err = client.CompareEventReports(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "0.5", response.Msg.GetDifferences()[2].GetBefore())
	version = 99
	_, err = client.CompareEventReports(context.Background(), request)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	request.Msg.PlanVersionA = nil
	request.Msg.EventIdB = "event-unpublished"
	_, err = client.CompareEventReports(context.Background(), request)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, 0, source.liveReads)
}

func (source *reportSource) EventReportData(context.Context, string) (core.StoredEvent, error) {
	source.liveReads++
	return source.live, nil
}

func TestGetEventReportReturnsStoredAndLiveViews(t *testing.T) {
	source := &reportSource{published: &core.EventReport{EventID: "event-1", PlanVersion: 2, Measurement: &core.Measurement{BaselineMW: 1.5}, Delivered: &core.Delivered{DeliveredMWh: 0.3, UncertainIntervals: []core.UncertainInterval{{DeviceID: "private-device"}}}}}
	_, handler := gridosv1connect.NewReportServiceHandler(NewService(source))
	server := httptest.NewServer(handler)
	defer server.Close()
	client := gridosv1connect.NewReportServiceClient(http.DefaultClient, server.URL)
	full := connect.NewRequest(&gridosv1.GetEventReportRequest{EventId: "event-1"})
	full.Header().Set("X-GridOS-Role", "analyst")
	response, err := client.GetEventReport(context.Background(), full)
	require.NoError(t, err)
	require.JSONEq(t, `{"EventID":"event-1","PlanVersion":2,"Measurement":{"BaselineMW":1.5}}`, selectedFullReportJSON(t, response.Msg.GetReportJson()))
	require.Contains(t, response.Msg.GetReportJson(), "private-device")
	require.Equal(t, 0, source.liveReads)
	partner := connect.NewRequest(&gridosv1.GetEventReportRequest{EventId: "event-1", PartnerView: true})
	partner.Header().Set("X-GridOS-Role", "partner")
	partnerResponse, err := client.GetEventReport(context.Background(), partner)
	require.NoError(t, err)
	require.NotContains(t, partnerResponse.Msg.GetReportJson(), "private-device")
	require.JSONEq(t, `{"event_id":"event-1","plan_version":2,"requested_mw":0,"approved_mw":0,"commanded_mw":0,"acknowledged_mw":0,"delivered_mw":0,"delivered_mwh":0.3}`, partnerResponse.Msg.GetReportJson())
	partner.Msg.PartnerView = false
	_, err = client.GetEventReport(context.Background(), partner)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	source.published = nil
	source.live = core.StoredEvent{PlanVersion: 2, RequestedMW: 1, Measurement: &core.Measurement{BaselineMW: 1.5, BaselineMethod: "load-baseline-v1", DeliveryMethod: "UNAVAILABLE"}, DataGaps: []core.DataGap{{Begin: time.Unix(1, 0), End: time.Unix(2, 0), Reason: "modeled_economics_unavailable"}}}
	response, err = client.GetEventReport(context.Background(), full)
	require.NoError(t, err)
	require.Contains(t, response.Msg.GetReportJson(), "modeled_economics_unavailable")
	require.Equal(t, 1, source.liveReads)
}

func selectedFullReportJSON(t *testing.T, encoded string) string {
	t.Helper()
	var value struct {
		EventID     string
		PlanVersion uint64
		Measurement struct{ BaselineMW float64 }
	}
	require.NoError(t, json.Unmarshal([]byte(encoded), &value))
	selected, err := json.Marshal(value)
	require.NoError(t, err)
	return string(selected)
}

func TestGetEventReportRequiresRoleAndEvent(t *testing.T) {
	service := NewService(&reportSource{})
	request := connect.NewRequest(&gridosv1.GetEventReportRequest{EventId: "event-1"})
	_, err := service.GetEventReport(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	request.Header().Set("X-GridOS-Role", "member")
	_, err = service.GetEventReport(context.Background(), request)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	request.Header().Set("X-GridOS-Role", "analyst")
	request.Msg.EventId = ""
	_, err = service.GetEventReport(context.Background(), request)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetEventReportReserveCompliance(t *testing.T) {
	margin := 0.5
	source := &reportSource{live: core.StoredEvent{PlanVersion: 1, RequestedMW: 1,
		ReserveCompliance: &core.ReserveCompliance{DevicesExpected: 2, DevicesObserved: 1, MinimumMarginKWh: &margin,
			ObservationGaps: 1, ValueKind: "MEASURED", Provenance: []string{"TELEMETRY_OBSERVATIONS", "FROZEN_EFFECTIVE_RESERVE"}}}}
	request := connect.NewRequest(&gridosv1.GetEventReportRequest{EventId: "event-reserve"})
	request.Header().Set("X-GridOS-Role", "analyst")
	response, err := NewService(source).GetEventReport(context.Background(), request)
	require.NoError(t, err)
	var report map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(response.Msg.GetReportJson()), &report))
	require.Equal(t, `"event-reserve"`, string(report["EventID"]))
	if _, found := report["ReserveCompliance"]; !found {
		t.Fatalf("measured reserve compliance missing from report: %s", response.Msg.GetReportJson())
	}
	require.Contains(t, string(report["ReserveCompliance"]), `"ValueKind":"MEASURED"`)
}

func TestGetEventReportShortfallSeparatesPlanAndMeasuredDelivery(t *testing.T) {
	source := &reportSource{}
	fixture := `{"PlanVersion":1,"RequestedMW":1,"planned_shortfall":[{"begin":"2026-09-27T10:00:00Z","end":"2026-09-27T10:05:00Z","requested_kw":1000,"feasible_kw":800,"shortfall_kw":200,"reasons":["RESERVE"]}],"delivery_shortfall":[{"begin":"2026-09-27T10:00:00Z","end":"2026-09-27T10:05:00Z","requested_kwh":83.33333333333333,"measured_delivered_kwh":0,"shortfall_kwh":83.33333333333333,"coverage":1,"value_kind":"MEASURED"},{"begin":"2026-09-27T10:05:00Z","end":"2026-09-27T10:10:00Z","requested_kwh":83.33333333333333,"coverage":0,"value_kind":"UNKNOWN"}]}`
	if err := json.Unmarshal([]byte(fixture), &source.live); err != nil {
		t.Fatal(err)
	}
	request := connect.NewRequest(&gridosv1.GetEventReportRequest{EventId: "event-shortfall"})
	request.Header().Set("X-GridOS-Role", "analyst")
	response, err := NewService(source).GetEventReport(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(response.Msg.GetReportJson()), &body); err != nil {
		t.Fatal(err)
	}
	if string(body["RequestedMW"]) != "1" {
		t.Fatalf("requested power control missing: %s", response.Msg.GetReportJson())
	}
	var planned, delivered []map[string]json.RawMessage
	if err := json.Unmarshal(body["planned_shortfall"], &planned); err != nil || len(planned) != 1 || string(planned[0]["shortfall_kw"]) != "200" {
		t.Fatalf("planned shortfall = %s", response.Msg.GetReportJson())
	}
	if err := json.Unmarshal(body["delivery_shortfall"], &delivered); err != nil || len(delivered) != 2 || string(delivered[0]["measured_delivered_kwh"]) != "0" || string(delivered[1]["coverage"]) != "0" || delivered[1]["shortfall_kwh"] != nil {
		t.Fatalf("delivery shortfall = %s", response.Msg.GetReportJson())
	}
}
