package geo

import (
	"sort"
	"strings"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func statesBySite(states []fleet.SiteState) map[string]fleet.SiteState {
	indexed := make(map[string]fleet.SiteState, len(states))
	for _, state := range states {
		indexed[state.SiteID] = state
	}
	return indexed
}

func metadataForSites(sites []*gridosv1.AuthorizedSite, states map[string]fleet.SiteState, now time.Time) *gridosv1.AggregateMetadata {
	counts := make(map[gridosv1.DataProvenance]uint64)
	complete := len(sites) > 0
	var freshness time.Duration
	for _, site := range sites {
		state, found := states[site.GetSite().GetSiteId()]
		provenance := site.GetSite().GetProvenance().GetProvenance()
		if state.Provenance != "" {
			value, valid := gridosv1.DataProvenance_value["DATA_PROVENANCE_"+strings.ToUpper(state.Provenance)]
			if valid {
				provenance = gridosv1.DataProvenance(value)
			} else {
				provenance = gridosv1.DataProvenance_DATA_PROVENANCE_UNSPECIFIED
			}
		}
		counts[provenance]++
		if !found || state.ObservedAt.IsZero() {
			complete = false
			continue
		}
		freshness = max(freshness, max(now.Sub(state.ObservedAt), 0))
	}
	metadata := &gridosv1.AggregateMetadata{Timestamp: timestamppb.New(now)}
	for provenance, count := range counts {
		metadata.ProvenanceMix = append(metadata.ProvenanceMix, &gridosv1.ProvenanceShare{Provenance: provenance, RecordCount: count})
	}
	sort.Slice(metadata.ProvenanceMix, func(i, j int) bool {
		return metadata.ProvenanceMix[i].Provenance < metadata.ProvenanceMix[j].Provenance
	})
	if complete {
		metadata.Freshness = durationpb.New(freshness)
	}
	return metadata
}

func singleProvenance(metadata *gridosv1.AggregateMetadata) gridosv1.DataProvenance {
	if len(metadata.GetProvenanceMix()) == 1 {
		return metadata.GetProvenanceMix()[0].GetProvenance()
	}
	return gridosv1.DataProvenance_DATA_PROVENANCE_UNSPECIFIED
}
