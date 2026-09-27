package context

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestContextServicePublicResponses(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	root := filepath.Join("..", "..", "..", "..", "..", "testdata", "fixtures", "public")
	service := NewService(root, func() time.Time { return now })
	market, err := service.GetMarketContext(context.Background(), operatorRequest(&gridosv1.GetMarketContextRequest{SettlementPoint: "HB_HOUSTON", WeatherZone: "COAST"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(market.Msg.GetDayAheadPrices()) == 0 || len(market.Msg.GetRealTimePrices()) == 0 || len(market.Msg.GetSystemLoads()) == 0 {
		t.Fatalf("market context = %#v", market.Msg)
	}
	assertPublicSource(t, market.Msg.GetDayAheadPrices()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	assertPublicSource(t, market.Msg.GetSystemLoads()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	weather, err := service.GetWeatherContext(context.Background(), operatorRequest(&gridosv1.GetWeatherContextRequest{City: "houston"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(weather.Msg.GetForecasts()) == 0 || len(weather.Msg.GetAlerts()) == 0 {
		t.Fatalf("weather context = %#v", weather.Msg)
	}
	assertPublicSource(t, weather.Msg.GetForecasts()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	assertPublicSource(t, weather.Msg.GetAlerts()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	outage, err := service.GetOutageRisk(context.Background(), operatorRequest(&gridosv1.GetOutageRiskRequest{County: "Harris"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(outage.Msg.GetRates()) != 1 {
		t.Fatalf("outage context = %#v", outage.Msg)
	}
	assertPublicSource(t, outage.Msg.GetRates()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED)
	begin := now.Add(time.Hour)
	windows, err := service.ListDispatchWindows(context.Background(), operatorRequest(&gridosv1.ListDispatchWindowsRequest{Candidates: []*gridosv1.ContextWindowCandidate{
		{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)), PriceUsdPerMwh: 50, RegionalLoadMw: 1000, OutageRisk: 0.2, FeasibleCapacityMw: 1},
		{BeginTime: timestamppb.New(begin.Add(time.Hour)), EndTime: timestamppb.New(begin.Add(2 * time.Hour)), PriceUsdPerMwh: 75, RegionalLoadMw: 1500, OutageRisk: 0.4, FeasibleCapacityMw: 1},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(windows.Msg.GetWindows()) != 2 || windows.Msg.GetWindows()[0].GetValueKind() != "modeled_estimate" || windows.Msg.GetWindows()[0].GetRank() != 1 {
		t.Fatalf("dispatch windows = %#v", windows.Msg)
	}
}

func TestGetMarketContextAustinRegion(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	root := filepath.Join("..", "..", "..", "..", "..", "testdata", "fixtures", "public")
	service := NewService(root, func() time.Time { return now })
	response, err := service.GetMarketContext(context.Background(), operatorRequest(&gridosv1.GetMarketContextRequest{SettlementPoint: "LZ_AEN", WeatherZone: "SOUTH_C"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.GetDayAheadPrices()) == 0 || len(response.Msg.GetRealTimePrices()) == 0 || len(response.Msg.GetSystemLoads()) == 0 {
		t.Fatalf("Austin market context = %#v", response.Msg)
	}
	for _, price := range append(response.Msg.GetDayAheadPrices(), response.Msg.GetRealTimePrices()...) {
		if price.GetSettlementPoint() != "LZ_AEN" || price.GetIntervalEnd() == nil || price.GetSource().GetAsOf() == nil {
			t.Fatalf("unlabelled Austin price = %#v", price)
		}
	}
	for _, load := range response.Msg.GetSystemLoads() {
		if load.GetWeatherZone() != "SOUTH_C" || load.GetIntervalEnd() == nil || load.GetSource().GetAsOf() == nil {
			t.Fatalf("unlabelled Austin load = %#v", load)
		}
	}
}

func TestGetWeatherContextSimulatedScenarioAlert(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	testdata, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, name := range []string{"ercot-prices", "system-load", "outages", "load-profiles", "fleet"} {
		if err := os.Symlink(filepath.Join(testdata, "fixtures", "public", name), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(testdata, "scenarios", "weather"), filepath.Join(root, "weather")); err != nil {
		t.Fatal(err)
	}
	response, err := NewService(root, func() time.Time { return now }).GetWeatherContext(context.Background(), operatorRequest(&gridosv1.GetWeatherContextRequest{City: "austin"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, alert := range response.Msg.GetAlerts() {
		if alert.GetSource().GetProvenance() == gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED {
			assertPublicSource(t, alert.GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED)
			return
		}
	}
	t.Fatalf("no simulated Austin alert in %#v", response.Msg.GetAlerts())
}

func TestContextServiceRejectsInvalidWindows(t *testing.T) {
	service := NewService("testdata/fixtures/public", time.Now)
	begin := time.Now().Add(time.Hour)
	valid := &gridosv1.ContextWindowCandidate{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)), PriceUsdPerMwh: 50, RegionalLoadMw: 1000, OutageRisk: 0.2, FeasibleCapacityMw: 1}
	if _, err := service.ListDispatchWindows(context.Background(), operatorRequest(&gridosv1.ListDispatchWindowsRequest{Candidates: []*gridosv1.ContextWindowCandidate{valid}})); err != nil {
		t.Fatalf("positive control window rejected: %v", err)
	}
	invalid := proto.Clone(valid).(*gridosv1.ContextWindowCandidate)
	invalid.PriceUsdPerMwh = math.NaN()
	if _, err := service.ListDispatchWindows(context.Background(), operatorRequest(&gridosv1.ListDispatchWindowsRequest{Candidates: []*gridosv1.ContextWindowCandidate{invalid}})); err == nil {
		t.Fatal("non-finite candidate accepted")
	}
}

func operatorRequest[T any](message *T) *connect.Request[T] {
	request := connect.NewRequest(message)
	request.Header().Set("X-GridOS-Role", "operator")
	return request
}

func assertPublicSource(t *testing.T, source *gridosv1.ContextSource, provenance gridosv1.DataProvenance) {
	t.Helper()
	if source == nil || source.GetProvenance() != provenance || source.GetAsOf() == nil || source.GetFreshness() == nil || source.GetFreshness().AsDuration() <= 0 {
		t.Fatalf("public source = %#v", source)
	}
}
