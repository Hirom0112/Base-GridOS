package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type explanationStore struct {
	EventStore
	request *gridosv1.OptimizationRequest
	plan    *gridosv1.DispatchPlan
}

func (store explanationStore) LoadPlan(_ context.Context, _ string, _ uint64) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return store.request, store.plan, nil
}

func TestGetPlanExplanation(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	events := NewMemoryEventStore()
	events.Put(&gridosv1.DispatchEvent{EventId: "event-explain", PlanVersion: 7, State: gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED})
	store := explanationStore{
		EventStore: events,
		request: &gridosv1.OptimizationRequest{Devices: []*gridosv1.DeviceState{
			{DeviceId: "a", EffectiveReserveKwh: 8},
			{DeviceId: "b", EffectiveReserveKwh: 12},
		}},
		plan: &gridosv1.DispatchPlan{
			EventId: "event-explain", PlanVersion: 7,
			MarginExplanation:  &gridosv1.MarginExplanation{ConservativeMargin: -0.1, Terms: []*gridosv1.MarginTerm{{Name: "DISPATCH_VALUE", Low: -0.1, Source: "FROZEN_PUBLIC_PRICE"}, {Name: "MEMBER_REWARD", Unavailable: true}}},
			ObjectiveBreakdown: &gridosv1.ObjectiveBreakdown{GridValue: 50, DegradationCost: 3},
			ConstraintMargins:  []*gridosv1.ConstraintMargin{{ConstraintName: "feeder", Margin: 4, Units: "kW"}},
			Exclusions:         []*gridosv1.DeviceExclusion{{DeviceId: "c", Reason: gridosv1.ExclusionReason_EXCLUSION_REASON_RESERVE}},
			Shortfalls: []*gridosv1.ShortfallReport{{
				IntervalBeginTime: timestamppb.New(now), IntervalEndTime: timestamppb.New(now.Add(5 * time.Minute)),
				RequestedKw: 100, FeasibleKw: 90, ShortfallKw: 10,
			}},
		},
	}
	service := NewService(store, nil, nil, func() time.Time { return now })
	server := httptest.NewServer(NewHandler(service))
	defer server.Close()
	client := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, server.URL)
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: "event-explain", PlanVersion: 7})
	request.Header().Set(roleHeader, "analyst")
	response, err := client.GetPlanExplanation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	explanation := response.Msg
	if explanation.GetObjectiveBreakdown().GetGridValue() != 50 || explanation.GetReserveHeldBackKwh() != 20 || explanation.GetConstraintMargins()[0].GetMargin() != 4 || explanation.GetExclusions()[0].GetDeviceId() != "c" || explanation.GetShortfalls()[0].GetShortfallKw() != 10 {
		t.Fatalf("explanation = %#v", explanation)
	}
	if explanation.GetMarginExplanation().GetConservativeMargin() != -0.1 || explanation.GetMarginExplanation().GetTerms()[0].GetSource() != "FROZEN_PUBLIC_PRICE" || !explanation.GetMarginExplanation().GetTerms()[1].GetUnavailable() {
		t.Fatalf("margin explanation = %#v", explanation.GetMarginExplanation())
	}
	request.Header().Set(roleHeader, "member")
	_, err = client.GetPlanExplanation(context.Background(), request)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("member explanation code = %v", connect.CodeOf(err))
	}
	request.Header().Set(roleHeader, "analyst")
	request.Msg.PlanVersion = 6
	_, err = client.GetPlanExplanation(context.Background(), request)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("stale version code = %v", connect.CodeOf(err))
	}
}

func TestGetPlanExplanationManifest(t *testing.T) {
	pool := apiTestDatabase(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO dispatch_requests
		(request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-manifest', 'GRID_SERVICE', now(), now() + interval '1 hour', 100, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'manifest');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-manifest', 'request-manifest', 'VALIDATED', 2, 'manifest');
		INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ('input-manifest', 'event-manifest', now(), '{"reservePolicy":{"policyVersion":"input-policy"}}', '{"code_version":"build-frozen-1"}', 'manifest');
		INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		VALUES ('eligible-manifest', 'event-manifest', now(), ARRAY[]::text[], '[]', 'policy-frozen-1', 'manifest');
		INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-manifest', 2, 'input-manifest', 'eligible-manifest', '{"eventId":"event-manifest","planVersion":"2","objectiveBreakdown":{"gridValue":4}}', 'solver-frozen-1', 'model-frozen-1', 'manifest')`)
	if err != nil {
		t.Fatal(err)
	}
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: "event-manifest", PlanVersion: 2})
	request.Header().Set(roleHeader, "analyst")
	response, err := NewService(NewPostgresEventStore(pool), nil, nil, time.Now).GetPlanExplanation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetObjectiveBreakdown().GetGridValue() != 4 {
		t.Fatal("stored plan control missing")
	}
	encoded, err := protojson.Marshal(response.Msg)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Manifest struct {
			InputSnapshotID       string `json:"inputSnapshotId"`
			EligibilitySnapshotID string `json:"eligibilitySnapshotId"`
			PolicyVersion         string `json:"policyVersion"`
			SolverVersion         string `json:"solverVersion"`
			ModelVersion          string `json:"modelVersion"`
			CodeVersion           string `json:"codeVersion"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	manifest := value.Manifest
	if manifest.InputSnapshotID != "input-manifest" || manifest.EligibilitySnapshotID != "eligible-manifest" || manifest.PolicyVersion != "policy-frozen-1" || manifest.SolverVersion != "solver-frozen-1" || manifest.ModelVersion != "model-frozen-1" || manifest.CodeVersion != "build-frozen-1" {
		t.Fatalf("frozen manifest = %s", encoded)
	}
}

func TestGetPlanExplanationFrozenEvidence(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	events := NewMemoryEventStore()
	events.Put(&gridosv1.DispatchEvent{EventId: "event-frozen", PlanVersion: 3})
	store := explanationStore{
		EventStore: events,
		request: &gridosv1.OptimizationRequest{Forecast: &gridosv1.ForecastResponse{SiteLoads: []*gridosv1.ForecastSiteLoad{{
			SiteId: "site-1", IntervalBeginTime: timestamppb.New(now), LoadKwh: &gridosv1.ForecastValue{
				Value: 2, Lower: 1, Upper: 3, ValueKind: "modeled_estimate", Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED,
				IssuedAt: timestamppb.New(now.Add(-time.Hour)), ModelVersion: "load-v1",
			},
		}}}, Devices: []*gridosv1.DeviceState{{DeviceId: "a", EffectiveReserveKwh: 8}}},
		plan: &gridosv1.DispatchPlan{EventId: "event-frozen", PlanVersion: 3,
			FallbackUsed: true, FallbackReason: "solver timeout",
			DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "a", Intervals: []*gridosv1.DeviceScheduleInterval{{
				BeginTime: timestamppb.New(now), EndTime: timestamppb.New(now.Add(5 * time.Minute)), SetpointKw: 3,
			}}}},
		},
	}
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: "event-frozen", PlanVersion: 3})
	request.Header().Set(roleHeader, "analyst")
	response, err := NewService(store, nil, nil, func() time.Time { return now }).GetPlanExplanation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetReserveHeldBackKwh() != 8 {
		t.Fatalf("frozen input not loaded: %#v", response.Msg)
	}
	encoded, err := protojson.Marshal(response.Msg)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Evidence struct {
			SiteLoads []struct {
				SiteID            string `json:"siteId"`
				IntervalBeginTime string `json:"intervalBeginTime"`
				LoadKwh           struct {
					Value      float64 `json:"value"`
					ValueKind  string  `json:"valueKind"`
					Provenance string  `json:"provenance"`
					IssuedAt   string  `json:"issuedAt"`
				} `json:"loadKwh"`
			} `json:"siteLoads"`
			SiteLoadUnits   string `json:"siteLoadUnits"`
			FallbackUsed    bool   `json:"fallbackUsed"`
			FallbackReason  string `json:"fallbackReason"`
			DeviceSchedules []struct {
				DeviceID string `json:"deviceId"`
			} `json:"deviceSchedules"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	evidence := decoded.Evidence
	if len(evidence.SiteLoads) != 1 {
		t.Fatalf("frozen site loads = %s", encoded)
	}
	load := evidence.SiteLoads[0]
	if load.SiteID != "site-1" || load.IntervalBeginTime == "" || load.LoadKwh.Value != 2 || load.LoadKwh.ValueKind != "modeled_estimate" || load.LoadKwh.Provenance != "DATA_PROVENANCE_SIMULATED" || load.LoadKwh.IssuedAt == "" || evidence.SiteLoadUnits != "kWh" {
		t.Fatalf("frozen forecast = %s", encoded)
	}
	if !evidence.FallbackUsed || evidence.FallbackReason != "solver timeout" || len(evidence.DeviceSchedules) != 1 || evidence.DeviceSchedules[0].DeviceID != "a" {
		t.Fatalf("frozen plan = %s", encoded)
	}
}

func TestGetPlanExplanationFrozenRegionalEvidence(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	value := &gridosv1.ForecastValue{Value: 0.8, ValueKind: "modeled_estimate", Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, IssuedAt: timestamppb.New(now.Add(-time.Hour))}
	forecast := &gridosv1.ForecastResponse{
		RegionalPrices:     []*gridosv1.ForecastRegionalPrice{{LoadZone: "LZ_AEN", IntervalBeginTime: timestamppb.New(now), PricePerMwh: value}},
		OutageRisks:        []*gridosv1.ForecastOutageRisk{{County: "Travis", IntervalBeginTime: timestamppb.New(now), Probability: value}},
		DeviceAvailability: []*gridosv1.ForecastDeviceAvailability{{DeviceId: "device-1", IntervalBeginTime: timestamppb.New(now), Probability: value}},
		UnavailableSources: []string{"weather"},
	}
	events := NewMemoryEventStore()
	events.Put(&gridosv1.DispatchEvent{EventId: "event-regional", PlanVersion: 1})
	store := explanationStore{EventStore: events, request: &gridosv1.OptimizationRequest{Forecast: forecast}, plan: &gridosv1.DispatchPlan{EventId: "event-regional", PlanVersion: 1}}
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: "event-regional", PlanVersion: 1})
	request.Header().Set(roleHeader, "analyst")
	response, err := NewService(store, nil, nil, func() time.Time { return now }).GetPlanExplanation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := protojson.Marshal(forecast)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, err := protojson.Marshal(response.Msg.GetEvidence())
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual map[string]json.RawMessage
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"regionalPrices", "outageRisks", "deviceAvailability", "unavailableSources"} {
		if !reflect.DeepEqual(actual[field], expected[field]) {
			t.Fatalf("frozen %s = %s, want %s", field, actual[field], expected[field])
		}
	}
}

func TestGetPlanExplanationReserveBasisUsesFrozenEligibilityAndPlan(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	events := NewMemoryEventStore()
	events.Put(&gridosv1.DispatchEvent{EventId: "event-reserve-basis", PlanVersion: 1})
	eligibility := new(gridosv1.EligibilitySnapshot)
	if err := protojson.Unmarshal([]byte(`{"reserveBases":[{"deviceId":"device-1","hardwareFloorKwh":1,"planReserveKwh":6,"overrideFloorKwh":8,"overrideReason":"RESERVE_OVERRIDE_REASON_WEATHER","overrideSourceId":"alert-1","policyVersion":"policy-v1","effectiveReserveKwh":8,"provenance":"DATA_PROVENANCE_SIMULATED","issuedAt":"2026-09-26T12:00:00Z"}],"travelFlexBindings":[{"windowId":"window-1","siteId":"site-1","creditType":"TRAVEL_FLEX_CREDIT_TYPE_FIXED_EVENT","creditCents":"500","provenance":"DATA_PROVENANCE_SIMULATED","issuedAt":"2026-09-26T12:00:00Z"}]}`), eligibility); err != nil {
		t.Fatal(err)
	}
	store := explanationStore{EventStore: events,
		request: &gridosv1.OptimizationRequest{Devices: []*gridosv1.DeviceState{{DeviceId: "device-1", EffectiveReserveKwh: 8}}, EligibilitySnapshot: eligibility},
		plan:    &gridosv1.DispatchPlan{EventId: "event-reserve-basis", PlanVersion: 1, DeviceSchedules: []*gridosv1.DeviceSchedule{{DeviceId: "device-1", ReserveSelection: gridosv1.ReserveSelection_RESERVE_SELECTION_TRAVEL_FLEX, SelectedReserveKwh: 8}}},
	}
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: "event-reserve-basis", PlanVersion: 1})
	request.Header().Set(roleHeader, "analyst")
	response, err := NewService(store, nil, nil, func() time.Time { return now }).GetPlanExplanation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := protojson.Marshal(response.Msg.GetEvidence())
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		ReserveBases []struct {
			OverrideSourceID    string  `json:"overrideSourceId"`
			EffectiveReserveKwh float64 `json:"effectiveReserveKwh"`
		} `json:"reserveBases"`
		TravelFlexBindings []struct {
			CreditCents string `json:"creditCents"`
		} `json:"travelFlexBindings"`
	}
	if err := json.Unmarshal(encoded, &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence.ReserveBases) != 1 || evidence.ReserveBases[0].OverrideSourceID != "alert-1" || evidence.ReserveBases[0].EffectiveReserveKwh != 8 || len(evidence.TravelFlexBindings) != 1 || evidence.TravelFlexBindings[0].CreditCents != "500" {
		t.Fatalf("frozen reserve evidence = %s", encoded)
	}
}
