package api

import (
	"fmt"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestListSitesPrivacyMerge(t *testing.T) {
	sites := make([]*gridosv1.AuthorizedSite, 0, 9)
	for index := range 9 {
		cell := "87489e346ffffff"
		if index >= 3 {
			cell = "87489e341ffffff"
		}
		provenance := gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED
		if index == 8 {
			provenance = gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC
		}
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: fmt.Sprintf("site-%d", index), H3Cell: cell, Provenance: &gridosv1.Provenance{Provenance: provenance}}})
	}
	locations, err := aggregateSites(sites, nil, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 || locations[0].GetAggregate().GetSiteCount() != 9 || locations[0].GetAggregate().GetH3Cell() != "86489e347ffffff" {
		t.Fatalf("private aggregation = %+v", locations)
	}
	var provenanceTotal uint64
	for _, part := range locations[0].GetAggregate().GetInstalledMw().GetMetadata().GetProvenanceMix() {
		provenanceTotal += part.GetRecordCount()
	}
	if provenanceTotal != 9 {
		t.Fatalf("parent provenance covers %d sites, want 9", provenanceTotal)
	}
	locations, err = aggregateSites(sites[:4], nil, time.Unix(100, 0))
	if err != nil || len(locations) != 0 {
		t.Fatalf("small cohort leaked: %+v, %v", locations, err)
	}
	sites[0].Site.H3Cell = "invalid"
	if _, err := aggregateSites(sites, nil, time.Unix(100, 0)); err == nil {
		t.Fatal("invalid H3 cell accepted")
	}
}
