package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	apievents "github.com/Hirom0112/Base-GridOS/services/control/internal/api/events"
	apigeo "github.com/Hirom0112/Base-GridOS/services/control/internal/api/geo"
	apireplay "github.com/Hirom0112/Base-GridOS/services/control/internal/api/replay"
	apireport "github.com/Hirom0112/Base-GridOS/services/control/internal/api/report"
)

type roleMatrixCase struct {
	method  string
	allowed string
	body    string
}

func assertRoleMatrix(t *testing.T, server *httptest.Server, cases []roleMatrixCase) {
	t.Helper()
	roles := []string{"operator", "approver", "analyst", "partner", "service", "member", "", "unknown"}
	for _, entry := range cases {
		for _, role := range roles {
			body := entry.body
			if body == "" {
				body = "{}"
			}
			request, err := http.NewRequest(http.MethodPost, server.URL+"/gridos.v1."+entry.method, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(roleHeader, role)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			allowed := strings.Contains(" "+entry.allowed+" ", " "+role+" ") && role != ""
			if role == "" && response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s missing role status = %d, want 401", entry.method, response.StatusCode)
				continue
			}
			if role == "" {
				continue
			}
			if allowed && response.StatusCode == http.StatusForbidden {
				t.Errorf("%s %q rejected by role gate", entry.method, role)
			}
			if !allowed && response.StatusCode != http.StatusForbidden {
				t.Errorf("%s %q status = %d, want 403", entry.method, role, response.StatusCode)
			}
		}
	}
}

func TestRoleMatrixControl(t *testing.T) {
	server, _, _, _ := testServer(t)
	defer server.Close()
	assertRoleMatrix(t, server, []roleMatrixCase{
		{method: "FleetService/GetFleetSummary", allowed: "operator approver analyst partner service"},
		{method: "FleetService/ListSites", allowed: "operator approver analyst partner service"},
		{method: "DispatchService/CreateEventRequest", allowed: "operator"},
		{method: "DispatchService/GetEvent", allowed: "operator approver analyst partner service"},
		{method: "DispatchService/ApproveEvent", allowed: "approver"},
		{method: "DispatchService/LaunchEvent", allowed: "approver"},
		{method: "DispatchService/GetPlanExplanation", allowed: "operator approver analyst service"},
		{method: "DispatchService/ValidateUnsafeAlternative", allowed: "operator approver"},
	})
}

func TestRoleMatrixReadServices(t *testing.T) {
	mux := http.NewServeMux()
	geoPath, geoHandler := gridosv1connect.NewGeoServiceHandler(apigeo.NewService(nil, nil, time.Now))
	mux.Handle(geoPath, geoHandler)
	reportPath, reportHandler := gridosv1connect.NewReportServiceHandler(apireport.NewService(nil))
	mux.Handle(reportPath, reportHandler)
	replayPath, replayHandler := gridosv1connect.NewReplayServiceHandler(apireplay.NewService("", nil, nil, nil))
	mux.Handle(replayPath, replayHandler)
	eventsPath, eventsHandler := gridosv1connect.NewEventsServiceHandler(apievents.NewService(nil, time.Second))
	mux.Handle(eventsPath, eventsHandler)
	server := httptest.NewServer(mux)
	defer server.Close()
	assertRoleMatrix(t, server, []roleMatrixCase{
		{method: "GeoService/ListCells", allowed: "operator approver analyst partner service"},
		{method: "GeoService/Drilldown", allowed: "operator approver analyst partner service"},
		{method: "ReportService/GetEventReport", allowed: "operator approver analyst partner service", body: `{"partnerView":true}`},
		{method: "ReportService/CompareEventReports", allowed: "operator approver analyst service"},
		{method: "ReplayService/ReplayEvent", allowed: "operator approver analyst service"},
		{method: "EventsService/GetEventTimeline", allowed: "operator approver analyst service"},
		{method: "EventsService/EmergencyStop", allowed: "operator approver"},
	})
	client := gridosv1connect.NewEventsServiceClient(server.Client(), server.URL)
	for _, role := range []string{"operator", "approver", "analyst", "partner", "service", "member", "", "unknown"} {
		request := connect.NewRequest(&gridosv1.WatchEventRequest{})
		request.Header().Set(roleHeader, role)
		stream, err := client.WatchEvent(context.Background(), request)
		if err == nil {
			stream.Receive()
			err = stream.Err()
		}
		wanted := connect.CodePermissionDenied
		if role == "" {
			wanted = connect.CodeUnauthenticated
		} else if strings.Contains(" operator approver analyst service ", " "+role+" ") {
			wanted = connect.CodeInvalidArgument
		}
		if connect.CodeOf(err) != wanted {
			t.Errorf("EventsService/WatchEvent %q code = %v, want %v", role, connect.CodeOf(err), wanted)
		}
	}
}
