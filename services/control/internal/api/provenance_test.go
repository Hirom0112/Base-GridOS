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
)

func TestProvenanceOnEveryFleetAggregateAndEvent(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	provenance := &gridosv1.Provenance{Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, SourceId: "fleet-file", ObservedAt: timestamp(now), IngestedAt: timestamp(now), SchemaVersion: "1"}
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-1", ObservedAt: now, OperatingState: fleet.OnGrid, DispatchableKW: 5, DispatchableKWh: 10, Provenance: "simulated"})
	sites := []*gridosv1.AuthorizedSite{{
		Site:    &gridosv1.Site{SiteId: "site-1", LoadZone: "LZ_AEN", H3Cell: "87283472bffffff", Provenance: provenance},
		Devices: []*gridosv1.Device{{DeviceId: "device-1", SiteId: "site-1", Provenance: provenance, BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 39.2, MaxDischargeKw: 10}}},
	}}
	for _, id := range []string{"site-2", "site-3", "site-4", "site-5"} {
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: id, LoadZone: "LZ_AEN", H3Cell: "87283472bffffff", Provenance: provenance}})
	}
	server := httptest.NewServer(NewHandler(NewService(NewMemoryEventStore(), twin, sites, func() time.Time { return now })))
	defer server.Close()
	fleetClient := gridosv1connect.NewFleetServiceClient(http.DefaultClient, server.URL)
	dispatchClient := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, server.URL)

	summaryRequest := connect.NewRequest(&gridosv1.GetFleetSummaryRequest{})
	summaryRequest.Header().Set(roleHeader, "operator")
	summaryResponse, err := fleetClient.GetFleetSummary(context.Background(), summaryRequest)
	if err != nil {
		t.Fatal(err)
	}
	summary := summaryResponse.Msg.GetSummary()
	quantities := []*gridosv1.FleetQuantityAggregate{summary.GetInstalledMw(), summary.GetInstalledMwh(), summary.GetDispatchableNowMw(), summary.GetForecastDispatchableMw(), summary.GetReservedForBackupMwh()}
	for _, quantity := range quantities {
		assertProvenanceMix(t, quantity.GetMetadata())
	}
	for _, count := range summary.GetOperatingStateCounts() {
		assertProvenanceMix(t, count.GetAggregate().GetMetadata())
	}
	for _, count := range summary.GetAvailabilityStateCounts() {
		assertProvenanceMix(t, count.GetAggregate().GetMetadata())
	}
	for _, count := range summary.GetCommunicationsHealthCounts() {
		assertProvenanceMix(t, count.GetAggregate().GetMetadata())
	}
	for _, count := range summary.GetAcknowledgementHealthCounts() {
		assertProvenanceMix(t, count.GetAggregate().GetMetadata())
	}

	sitesRequest := connect.NewRequest(&gridosv1.ListSitesRequest{})
	sitesRequest.Header().Set(roleHeader, "operator")
	sitesResponse, err := fleetClient.ListSites(context.Background(), sitesRequest)
	if err != nil {
		t.Fatal(err)
	}
	if len(sitesResponse.Msg.GetSites()) != 1 || sitesResponse.Msg.GetSites()[0].GetAggregate().GetSiteCount() != 5 {
		t.Fatalf("private aggregate missing: %+v", sitesResponse.Msg.GetSites())
	}
	for _, location := range sitesResponse.Msg.GetSites() {
		assertProvenanceMix(t, location.GetAggregate().GetInstalledMw().GetMetadata())
		assertProvenanceMix(t, location.GetAggregate().GetInstalledMwh().GetMetadata())
	}

	create := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{RequestId: "provenance-event", BeginTime: timestamp(now), EndTime: timestamp(now.Add(time.Hour))}, IdempotencyKey: "provenance-create"})
	create.Header().Set(roleHeader, "operator")
	created, err := dispatchClient.CreateEventRequest(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	if created.Msg.GetEvent().GetProvenance().GetProvenance() != gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED {
		t.Fatalf("created event provenance = %#v", created.Msg.GetEvent().GetProvenance())
	}
	get := connect.NewRequest(&gridosv1.GetEventRequest{EventId: "provenance-event"})
	get.Header().Set(roleHeader, "operator")
	loaded, err := dispatchClient.GetEvent(context.Background(), get)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Msg.GetEvent().GetProvenance().GetProvenance() != gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED {
		t.Fatalf("loaded event provenance = %#v", loaded.Msg.GetEvent().GetProvenance())
	}
}

func assertProvenanceMix(t *testing.T, metadata *gridosv1.AggregateMetadata) {
	t.Helper()
	if metadata == nil || len(metadata.GetProvenanceMix()) == 0 || metadata.GetProvenanceMix()[0].GetRecordCount() == 0 {
		t.Fatalf("aggregate metadata has no provenance mix: %#v", metadata)
	}
}
