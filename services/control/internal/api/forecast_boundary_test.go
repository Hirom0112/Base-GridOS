package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type forecastHandler struct {
	gridosv1connect.UnimplementedOptimizationServiceHandler
	request *gridosv1.ForecastRequest
}

func (handler *forecastHandler) Forecast(_ context.Context, request *connect.Request[gridosv1.ForecastRequest]) (*connect.Response[gridosv1.ForecastResponse], error) {
	handler.request = request.Msg
	return connect.NewResponse(&gridosv1.ForecastResponse{UnavailableSources: []string{"county"}}), nil
}

func TestConnectOptimizerForecast(t *testing.T) {
	handler := &forecastHandler{}
	path, service := gridosv1connect.NewOptimizationServiceHandler(handler)
	mux := http.NewServeMux()
	mux.Handle(path, service)
	server := httptest.NewServer(mux)
	defer server.Close()
	optimizer := NewConnectOptimizer(gridosv1connect.NewOptimizationServiceClient(http.DefaultClient, server.URL))
	input := &gridosv1.ForecastRequest{Request: &gridosv1.OptimizationRequest{EventId: "event-forecast", PlanVersion: 3}}
	response, err := optimizer.Forecast(context.Background(), input)
	if err != nil || handler.request.GetRequest().GetEventId() != "event-forecast" || response.GetUnavailableSources()[0] != "county" {
		t.Fatalf("forecast boundary = %#v, %#v, %v", handler.request, response, err)
	}
}

func TestFleetSnapshotterForecastMetadata(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-forecast", ObservedAt: now, OperatingState: fleet.OnGrid, Availability: fleet.Online, EnergyKWh: 6, ReserveKWh: 4})
	sites := []*gridosv1.AuthorizedSite{{
		Site: &gridosv1.Site{SiteId: "site-forecast", LoadProfileType: "residential", ReliabilityTrait: "stable", LoadZone: "LZ_AEN", WeatherZone: "central"},
		Devices: []*gridosv1.Device{{DeviceId: "device-forecast", BatteryParameters: &gridosv1.BatteryParameters{
			UsableEnergyKwh: 10, MaxDischargeKw: 2, ChargeEfficiency: 1, DischargeEfficiency: 1,
		}}},
	}}
	snapshotter := NewFleetSnapshotter(twin, sites, func() time.Time { return now })
	frozen, err := snapshotter.Freeze(context.Background(), &gridosv1.DispatchEvent{EventId: "event-forecast"}, &gridosv1.EventRequest{
		RequestId: "request-forecast", BeginTime: timestamppb.New(now.Add(time.Minute)), EndTime: timestamppb.New(now.Add(6 * time.Minute)), TargetKw: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	site := frozen.Optimization.GetSites()
	device := frozen.Optimization.GetDevices()
	if len(site) != 1 || site[0].GetSiteId() != "site-forecast" || site[0].GetLoadProfileType() != "residential" || site[0].GetLoadZone() != "LZ_AEN" || site[0].GetWeatherZone() != "central" || site[0].GetCounty() != "" {
		t.Fatalf("frozen forecast site = %#v", site)
	}
	if len(device) != 1 || device[0].GetSiteId() != "site-forecast" || device[0].GetReliabilityTrait() != "stable" {
		t.Fatalf("frozen forecast device = %#v", device)
	}
}
