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
	published *core.EventReport
	live      core.StoredEvent
	liveReads int
}

func (source *reportSource) StoredReport(context.Context, string) (*core.EventReport, error) {
	return source.published, nil
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
