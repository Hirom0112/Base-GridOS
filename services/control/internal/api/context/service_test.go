package context

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestContextServicePublicResponses(t *testing.T) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	root := filepath.Join("..", "..", "..", "..", "..", "testdata", "fixtures", "public")
	service := NewService(root, func() time.Time { return now })
	market, err := service.GetMarketContext(context.Background(), connect.NewRequest(&gridosv1.GetMarketContextRequest{SettlementPoint: "HB_HOUSTON", WeatherZone: "COAST"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(market.Msg.GetDayAheadPrices()) == 0 || len(market.Msg.GetRealTimePrices()) == 0 || len(market.Msg.GetSystemLoads()) == 0 {
		t.Fatalf("market context = %#v", market.Msg)
	}
	assertPublicSource(t, market.Msg.GetDayAheadPrices()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	assertPublicSource(t, market.Msg.GetSystemLoads()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	weather, err := service.GetWeatherContext(context.Background(), connect.NewRequest(&gridosv1.GetWeatherContextRequest{City: "houston"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(weather.Msg.GetForecasts()) == 0 || len(weather.Msg.GetAlerts()) == 0 {
		t.Fatalf("weather context = %#v", weather.Msg)
	}
	assertPublicSource(t, weather.Msg.GetForecasts()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	assertPublicSource(t, weather.Msg.GetAlerts()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC)
	outage, err := service.GetOutageRisk(context.Background(), connect.NewRequest(&gridosv1.GetOutageRiskRequest{County: "Harris"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(outage.Msg.GetRates()) != 1 {
		t.Fatalf("outage context = %#v", outage.Msg)
	}
	assertPublicSource(t, outage.Msg.GetRates()[0].GetSource(), gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED)
	begin := now.Add(time.Hour)
	windows, err := service.ListDispatchWindows(context.Background(), connect.NewRequest(&gridosv1.ListDispatchWindowsRequest{Candidates: []*gridosv1.ContextWindowCandidate{
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

func TestContextServiceRejectsInvalidWindows(t *testing.T) {
	service := NewService("testdata/fixtures/public", time.Now)
	begin := time.Now().Add(time.Hour)
	valid := &gridosv1.ContextWindowCandidate{BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)), PriceUsdPerMwh: 50, RegionalLoadMw: 1000, OutageRisk: 0.2, FeasibleCapacityMw: 1}
	if _, err := service.ListDispatchWindows(context.Background(), connect.NewRequest(&gridosv1.ListDispatchWindowsRequest{Candidates: []*gridosv1.ContextWindowCandidate{valid}})); err != nil {
		t.Fatalf("positive control window rejected: %v", err)
	}
	invalid := *valid
	invalid.PriceUsdPerMwh = math.NaN()
	if _, err := service.ListDispatchWindows(context.Background(), connect.NewRequest(&gridosv1.ListDispatchWindowsRequest{Candidates: []*gridosv1.ContextWindowCandidate{&invalid}})); err == nil {
		t.Fatal("non-finite candidate accepted")
	}
}

func assertPublicSource(t *testing.T, source *gridosv1.ContextSource, provenance gridosv1.DataProvenance) {
	t.Helper()
	if source == nil || source.GetProvenance() != provenance || source.GetAsOf() == nil || source.GetFreshness() == nil || source.GetFreshness().AsDuration() <= 0 {
		t.Fatalf("public source = %#v", source)
	}
}
