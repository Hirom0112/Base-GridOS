package context

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	publiccontext "github.com/Hirom0112/Base-GridOS/services/control/internal/context"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	root string
	now  func() time.Time
}

var _ gridosv1connect.ContextServiceHandler = (*Service)(nil)

func NewService(root string, now func() time.Time) *Service {
	return &Service{root: root, now: now}
}

func (service *Service) snapshot() (publiccontext.Snapshot, error) {
	if service.root == "" || service.now == nil {
		return publiccontext.Snapshot{}, connect.NewError(connect.CodeUnavailable, errors.New("public context source is not configured"))
	}
	snapshot, err := publiccontext.LoadPublic(service.root, service.now())
	if err != nil {
		return publiccontext.Snapshot{}, connect.NewError(connect.CodeUnavailable, err)
	}
	return snapshot, nil
}

func (service *Service) GetMarketContext(_ context.Context, request *connect.Request[gridosv1.GetMarketContextRequest]) (*connect.Response[gridosv1.GetMarketContextResponse], error) {
	if request.Msg.GetSettlementPoint() == "" || request.Msg.GetWeatherZone() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("settlement point and weather zone are required"))
	}
	snapshot, err := service.snapshot()
	if err != nil {
		return nil, err
	}
	response := &gridosv1.GetMarketContextResponse{}
	for _, price := range snapshot.DayAheadPrices {
		if price.SettlementPoint == request.Msg.GetSettlementPoint() {
			response.DayAheadPrices = append(response.DayAheadPrices, marketPrice(price))
		}
	}
	for _, price := range snapshot.RealTimePrices {
		if price.SettlementPoint == request.Msg.GetSettlementPoint() {
			response.RealTimePrices = append(response.RealTimePrices, marketPrice(price))
		}
	}
	for _, load := range snapshot.SystemLoads {
		if load.Zone == request.Msg.GetWeatherZone() {
			response.SystemLoads = append(response.SystemLoads, &gridosv1.ContextSystemLoad{IntervalEnd: timestamppb.New(load.At), WeatherZone: load.Zone, Mw: load.MW, Source: contextSource(load.Source)})
		}
	}
	if len(response.DayAheadPrices) == 0 || len(response.RealTimePrices) == 0 || len(response.SystemLoads) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("market context is unavailable for the requested region"))
	}
	return connect.NewResponse(response), nil
}

func (service *Service) GetWeatherContext(_ context.Context, request *connect.Request[gridosv1.GetWeatherContextRequest]) (*connect.Response[gridosv1.GetWeatherContextResponse], error) {
	if request.Msg.GetCity() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("city is required"))
	}
	snapshot, err := service.snapshot()
	if err != nil {
		return nil, err
	}
	response := &gridosv1.GetWeatherContextResponse{}
	for _, forecast := range snapshot.Forecasts {
		if forecast.City == request.Msg.GetCity() {
			response.Forecasts = append(response.Forecasts, &gridosv1.ContextWeatherForecast{
				City: forecast.City, BeginTime: timestamppb.New(forecast.Start), EndTime: timestamppb.New(forecast.End),
				TemperatureF: int32(forecast.TemperatureF), Summary: forecast.Summary, Source: contextSource(forecast.Source),
			})
		}
	}
	for _, alert := range snapshot.Alerts {
		if alert.City == request.Msg.GetCity() {
			response.Alerts = append(response.Alerts, &gridosv1.ContextWeatherAlert{
				City: alert.City, Event: alert.Event, Severity: alert.Severity,
				EffectiveAt: timestamppb.New(alert.Effective), ExpiresAt: timestamppb.New(alert.Expires), Source: contextSource(alert.Source),
			})
		}
	}
	if len(response.Forecasts) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("weather context is unavailable for the requested city"))
	}
	return connect.NewResponse(response), nil
}

func (service *Service) GetOutageRisk(_ context.Context, request *connect.Request[gridosv1.GetOutageRiskRequest]) (*connect.Response[gridosv1.GetOutageRiskResponse], error) {
	if request.Msg.GetCounty() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("county is required"))
	}
	snapshot, err := service.snapshot()
	if err != nil {
		return nil, err
	}
	response := &gridosv1.GetOutageRiskResponse{}
	for _, rate := range snapshot.OutageRates {
		if rate.County == request.Msg.GetCounty() {
			response.Rates = append(response.Rates, &gridosv1.ContextOutageRate{County: rate.County, Month: rate.Month, Rate: rate.Rate, Source: contextSource(rate.Source)})
		}
	}
	if len(response.Rates) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("outage rate is unavailable for the requested county"))
	}
	return connect.NewResponse(response), nil
}

func (service *Service) ListDispatchWindows(_ context.Context, request *connect.Request[gridosv1.ListDispatchWindowsRequest]) (*connect.Response[gridosv1.ListDispatchWindowsResponse], error) {
	if len(request.Msg.GetCandidates()) > 1000 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("too many dispatch candidates"))
	}
	candidates := make([]publiccontext.CandidateWindow, 0, len(request.Msg.GetCandidates()))
	for _, candidate := range request.Msg.GetCandidates() {
		if candidate.GetBeginTime() == nil || candidate.GetEndTime() == nil || candidate.GetBeginTime().CheckValid() != nil || candidate.GetEndTime().CheckValid() != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("candidate interval timestamps are required"))
		}
		candidates = append(candidates, publiccontext.CandidateWindow{
			Begin: candidate.GetBeginTime().AsTime(), End: candidate.GetEndTime().AsTime(),
			PriceUSDPerMWh: candidate.GetPriceUsdPerMwh(), RegionalLoadMW: candidate.GetRegionalLoadMw(),
			OutageRisk: candidate.GetOutageRisk(), FeasibleCapacityMW: candidate.GetFeasibleCapacityMw(),
		})
	}
	windows, err := publiccontext.RankWindows(candidates)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	response := &gridosv1.ListDispatchWindowsResponse{Windows: make([]*gridosv1.ContextDispatchWindow, 0, len(windows))}
	for _, window := range windows {
		response.Windows = append(response.Windows, &gridosv1.ContextDispatchWindow{
			BeginTime: timestamppb.New(window.Begin), EndTime: timestamppb.New(window.End), Rank: uint32(window.Rank),
			Score: window.Score, ForecastGridValueUsd: window.ForecastGridValueUSD, ValueKind: window.ValueKind,
		})
	}
	return connect.NewResponse(response), nil
}
