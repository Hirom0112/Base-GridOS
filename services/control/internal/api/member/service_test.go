package member

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func memberDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("GRIDOS_DATABASE_URL")
	if url == "" {
		url = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_member_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migrations, err := filepath.Glob("../../../../../database/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(migrations)
	for _, path := range migrations {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, readErr = pool.Exec(ctx, string(contents)); readErr != nil {
			t.Fatalf("%s: %v", path, readErr)
		}
	}
	return pool
}

func TestMemberAwayCommandsAreScopedAndIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-1', 'member-1', now(), 'SIMULATED', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(pool, fleet.NewTwin(time.Minute), nil, time.Now)
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	request := connect.NewRequest(&gridosv1.ScheduleAwayRequest{MemberId: "member-1", IdempotencyKey: "away-1",
		StartTime: timestamppb.New(start), EndTime: timestamppb.New(start.Add(2 * time.Hour)),
		ConsentVersion: "consent-v1", CorrelationId: "corr-1"})
	request.Header().Set("X-GridOS-Role", "member")
	request.Header().Set("X-GridOS-Member-ID", "member-1")
	for range 2 {
		response, err := service.ScheduleAway(ctx, request)
		if err != nil || response.Msg.GetAwayPeriodId() != "away-1" {
			t.Fatalf("own away command: %v, %+v", err, response)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM member_away_periods WHERE away_period_id = 'away-1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("away rows = %d, %v", count, err)
	}
	request.Msg.IdempotencyKey = "away-2"
	request.Header().Set("X-GridOS-Member-ID", "member-2")
	if _, err := service.ScheduleAway(ctx, request); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("other member command code = %v", connect.CodeOf(err))
	}
}

func TestMemberStatusScopeAndSiteState(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-1', 'member-1', now(), 'SIMULATED', '{"provenance":"SIMULATED"}'),
		('site-2', 'member-2', now(), 'SIMULATED', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-1", ObservedAt: now, OperatingState: fleet.OnGrid,
		Availability: fleet.Online, EnergyKWh: 8, BackupHoursCurrent: 4, BackupHours750W: 8})
	sites := []*gridosv1.AuthorizedSite{{Site: &gridosv1.Site{SiteId: "site-1"}, Devices: []*gridosv1.Device{{DeviceId: "device-1", BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 10}}}}}
	service := NewService(pool, twin, sites, func() time.Time { return now })
	request := connect.NewRequest(&gridosv1.GetMemberStatusRequest{MemberId: "member-1", SiteId: "site-1"})
	request.Header().Set("X-GridOS-Role", "member")
	request.Header().Set("X-GridOS-Member-ID", "member-1")
	response, err := service.GetMemberStatus(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetStateOfEnergyPercent() != 80 || response.Msg.GetBackupHoursCurrent() != 4 || response.Msg.GetBackupHours_750W() != 8 {
		t.Fatalf("own site state = %+v", response.Msg)
	}
	request.Msg.SiteId = "site-2"
	if _, err := service.GetMemberStatus(ctx, request); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-member site code = %v", connect.CodeOf(err))
	}
	request.Msg.SiteId = "site-1"
	request.Header().Set("X-GridOS-Member-ID", "member-2")
	if _, err := service.GetMemberStatus(ctx, request); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-member principal code = %v", connect.CodeOf(err))
	}
	request.Header().Del("X-GridOS-Role")
	if _, err := service.GetMemberStatus(ctx, request); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("missing role code = %v", connect.CodeOf(err))
	}
}

func TestMemberStatusRecentEventsContainOwnDeviceOnly(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-1', 'member-1', now(), 'SIMULATED', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Now().UTC().Truncate(time.Second)
	for _, fixture := range []struct{ event, device string }{{"event-own", "device-1"}, {"event-other", "device-2"}} {
		_, err = pool.Exec(ctx, `INSERT INTO dispatch_requests(request_id,event_type,begin_time,end_time,target_kw,measurement_boundary,load_zones,correlation_id)
			VALUES ($1,'DEMAND_RESPONSE',$2,$3,1,'GATEWAY',ARRAY['LZ_AEN'],'corr')`, fixture.event, begin, begin.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO dispatch_events(event_id,request_id,state,plan_version,correlation_id)
			VALUES ($1,$1,'PLANNED',1,'corr')`, fixture.event)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO input_snapshots(snapshot_id,event_id,captured_at,inputs,provenance,correlation_id)
			VALUES ($1,$2,$3,'{}','{}','corr')`, fixture.event+":input", fixture.event, begin)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO eligibility_snapshots(snapshot_id,event_id,captured_at,eligible_device_ids,exclusions,policy_version,correlation_id)
			VALUES ($1,$2,$3,ARRAY[$4::text],'{}','policy-v1','corr')`, fixture.event+":eligibility", fixture.event, begin, fixture.device)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO plan_versions(event_id,version,input_snapshot_id,eligibility_snapshot_id,plan,solver_version,model_version,correlation_id)
			VALUES ($1,1,$2,$3,jsonb_build_object('deviceSchedules',jsonb_build_array(jsonb_build_object('deviceId',$4::text))),'solver-v1','model-v1','corr')`,
			fixture.event, fixture.event+":input", fixture.event+":eligibility", fixture.device)
		if err != nil {
			t.Fatal(err)
		}
	}
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-1", ObservedAt: begin, OperatingState: fleet.OnGrid, Availability: fleet.Online})
	sites := []*gridosv1.AuthorizedSite{{Site: &gridosv1.Site{SiteId: "site-1"}, Devices: []*gridosv1.Device{{DeviceId: "device-1"}}}}
	service := NewService(pool, twin, sites, func() time.Time { return begin })
	request := connect.NewRequest(&gridosv1.GetMemberStatusRequest{MemberId: "member-1", SiteId: "site-1"})
	request.Header().Set("X-GridOS-Role", "member")
	request.Header().Set("X-GridOS-Member-ID", "member-1")
	response, err := service.GetMemberStatus(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.GetRecentEvents()) != 1 || response.Msg.GetRecentEvents()[0].GetEventId() != "event-own" || !response.Msg.GetRecentEvents()[0].GetParticipated() {
		t.Fatalf("member events = %+v", response.Msg.GetRecentEvents())
	}
}

func TestMemberOfferSelectionUpdatesStatusReserve(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	begin := time.Now().UTC().Truncate(time.Second)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-1', 'member-1', now(), 'SIMULATED', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO reserve_policies(policy_version,protected_hardware_floor_percent,member_plan_floor_percent,dynamic_override_percent,effective_reserve_percent,effective_at,correlation_id)
		VALUES ('policy-v1',10,65,0,65,$1,'fixture')`, begin.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO pricing_catalog_snapshots(catalog_version,member_plan_id,market,display_name,reserve_floor_percent,energy_plan,energy_term_months,energy_monthly_charge_cents,battery_plan,battery_term_months,battery_monthly_charge_cents,flexibility_reward_cents,effective_at,correlation_id)
		VALUES ('catalog-v1','plan-1','TX','Cedar',65,'{}',0,1999,'{}',0,1500,500,$1,'fixture')`, begin.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO offer_terms(catalog_version,member_plan_id,kind,policy_version,contract_version,consent_version,consent_text,price_text,fixed_credit_cents)
		VALUES ('catalog-v1','plan-1','PLAN','policy-v1','contract-v1','consent-v1','Cedar reserve accepted','Cedar price',0)`)
	if err != nil {
		t.Fatal(err)
	}
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-1", ObservedAt: begin, OperatingState: fleet.OnGrid, Availability: fleet.Online})
	sites := []*gridosv1.AuthorizedSite{{Site: &gridosv1.Site{SiteId: "site-1"}}}
	service := NewService(pool, twin, sites, func() time.Time { return begin })
	offerRequest := connect.NewRequest(&gridosv1.PresentOfferRequest{MemberId: "member-1", IdempotencyKey: "offer-1",
		Kind: gridosv1.MemberOfferKind_MEMBER_OFFER_KIND_PLAN, Market: "TX", CatalogVersion: "catalog-v1",
		MemberPlanId: "plan-1", ContractVersion: "contract-v1", PriceText: "Cedar price",
		ConsentText: "Cedar reserve accepted", ConsentVersion: "consent-v1",
		EffectiveAt: timestamppb.New(begin.Add(-time.Minute)), ExpiresAt: timestamppb.New(begin.Add(time.Hour)), CorrelationId: "corr-1"})
	offerRequest.Header().Set("X-GridOS-Role", "member")
	offerRequest.Header().Set("X-GridOS-Member-ID", "member-1")
	offer, err := service.PresentOffer(ctx, offerRequest)
	if err != nil || offer.Msg.GetOffer().GetEnergyMonthlyChargeCents() != 1999 {
		t.Fatalf("presented offer: %v, %+v", err, offer)
	}
	selectionRequest := connect.NewRequest(&gridosv1.SelectResiliencePlanRequest{MemberId: "member-1",
		IdempotencyKey: "selection-1", OfferId: "offer-1", Market: "TX", CatalogVersion: "catalog-v1",
		MemberPlanId: "plan-1", PolicyVersion: "policy-v1", ConsentText: "Cedar reserve accepted",
		ConsentVersion: "consent-v1", ExplanationShown: "Reserve limits dispatch",
		EffectiveAt: timestamppb.New(begin), CorrelationId: "corr-1"})
	selectionRequest.Header().Set("X-GridOS-Role", "member")
	selectionRequest.Header().Set("X-GridOS-Member-ID", "member-1")
	selected, err := service.SelectResiliencePlan(ctx, selectionRequest)
	if err != nil || selected.Msg.GetPlan().GetReserveFloorPercent() != 65 {
		t.Fatalf("selected plan: %v, %+v", err, selected)
	}
	statusRequest := connect.NewRequest(&gridosv1.GetMemberStatusRequest{MemberId: "member-1", SiteId: "site-1"})
	statusRequest.Header().Set("X-GridOS-Role", "member")
	statusRequest.Header().Set("X-GridOS-Member-ID", "member-1")
	status, err := service.GetMemberStatus(ctx, statusRequest)
	if err != nil || status.Msg.GetCurrentPlan().GetOfferId() != "offer-1" || status.Msg.GetEffectiveReservePercent() != 65 {
		t.Fatalf("current member reserve: %v, %+v", err, status)
	}
}

func TestListHomeActivityAlertsReturnsOwnSignalsOnly(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id,member_id,bound_at,source,provenance)
		VALUES ('site-1','member-1',$1,'SIMULATED','{"provenance":"SIMULATED"}'),
		('site-2','member-2',$1,'SIMULATED','{"provenance":"SIMULATED"}')`, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, memberID := range []string{"member-1", "member-2"} {
		_, err = pool.Exec(ctx, `INSERT INTO member_anomaly_preferences(preference_id,member_id,opted_in,consent_text,
			consent_version,baseline_upper_kw,baseline_begin,baseline_end,effective_at,expires_at,correlation_id)
			VALUES ($1,$2,true,'away alert consent','consent-v1',1,$3,$4,$3,$4,$1)`, memberID+":preference", memberID, now.Add(-time.Hour), now.Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO member_alerts(alert_id,member_id,kind,message,preference_id,evidence,observed_at,correlation_id)
			VALUES ($1,$2,'ENERGY_ANOMALY_SIGNAL','energy anomaly signal',$3,
			'{"observation_id":"own-meter","site_id":"own-site","to_home_kw":2,"baseline_upper_kw":1,"consent_version":"consent-v1"}',$4,$1)`,
			memberID+":alert", memberID, memberID+":preference", now)
		if err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(pool, fleet.NewTwin(time.Minute), nil, time.Now)
	request := connect.NewRequest(&gridosv1.ListHomeActivityAlertsRequest{MemberId: "member-1"})
	request.Header().Set("X-GridOS-Role", "member")
	request.Header().Set("X-GridOS-Member-ID", "member-1")
	response, err := service.ListHomeActivityAlerts(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.GetAlerts()) != 1 || response.Msg.GetAlerts()[0].GetAlertId() != "member-1:alert" || response.Msg.GetAlerts()[0].GetDescription() != "energy anomaly signal" {
		t.Fatalf("own alert list = %+v", response.Msg.GetAlerts())
	}
	request.Header().Set("X-GridOS-Member-ID", "member-2")
	if _, err := service.ListHomeActivityAlerts(ctx, request); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-member alert code = %v", connect.CodeOf(err))
	}
}
