package geo

import (
	"errors"
	"sort"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	h3 "github.com/uber/h3-go/v4"
)

func drilldown(sites []*gridosv1.AuthorizedSite, parentID string, exact bool, states map[string]fleet.SiteState, now time.Time) (*gridosv1.DrilldownResponse, error) {
	children := make(map[string]map[string]*gridosv1.GeoNode)
	groups := make(map[string][]*gridosv1.AuthorizedSite)
	for _, site := range sites {
		path, err := hierarchyPath(site)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		parent := ""
		for _, node := range path {
			if children[parent] == nil {
				children[parent] = make(map[string]*gridosv1.GeoNode)
			}
			if children[parent][node.Id] == nil {
				children[parent][node.Id] = node
			}
			children[parent][node.Id].SiteCount++
			groups[node.Id] = append(groups[node.Id], site)
			parent = node.Id
		}
	}
	if parentID != "" && len(groups[parentID]) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("geo node not found"))
	}
	if parentID != "" && !exact && len(groups[parentID]) < 5 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("geo node not found"))
	}
	response := &gridosv1.DrilldownResponse{}
	selected := sites
	if parentID != "" {
		selected = groups[parentID]
	}
	response.Metadata = metadataForSites(selected, states, now)
	response.AsOf = response.Metadata.GetTimestamp()
	response.Freshness = response.Metadata.GetFreshness()
	if len(children[parentID]) == 0 && parentID != "" {
		if !exact {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("site_location permission required"))
		}
		response.Sites = groups[parentID]
		sort.Slice(response.Sites, func(i, j int) bool {
			return response.Sites[i].GetSite().GetSiteId() < response.Sites[j].GetSite().GetSiteId()
		})
		return response, nil
	}
	for _, node := range children[parentID] {
		if exact || node.SiteCount >= 5 {
			node.Metadata = metadataForSites(groups[node.Id], states, now)
			node.AsOf = node.Metadata.GetTimestamp()
			node.Freshness = node.Metadata.GetFreshness()
			response.Nodes = append(response.Nodes, node)
		}
	}
	sort.Slice(response.Nodes, func(i, j int) bool { return response.Nodes[i].Id < response.Nodes[j].Id })
	return response, nil
}

func hierarchyPath(site *gridosv1.AuthorizedSite) ([]*gridosv1.GeoNode, error) {
	identity := site.GetSite()
	cell := h3.CellFromString(identity.GetH3Cell())
	if identity.GetSiteId() == "" || identity.GetLoadZone() == "" || !cell.IsValid() || cell.Resolution() != 7 {
		return nil, errors.New("geo hierarchy requires site ID, load zone, and resolution-7 H3 cell")
	}
	substation, err := cell.Parent(5)
	if err != nil {
		return nil, err
	}
	feeder, err := cell.Parent(6)
	if err != nil {
		return nil, err
	}
	zone := identity.GetLoadZone()
	ids := []string{"market:ERCOT", "load_zone:" + zone, "utility:" + zone, "substation:" + zone + ":" + substation.String(), "feeder:" + zone + ":" + feeder.String()}
	levels := []gridosv1.GeoLevel{
		gridosv1.GeoLevel_GEO_LEVEL_MARKET,
		gridosv1.GeoLevel_GEO_LEVEL_LOAD_ZONE,
		gridosv1.GeoLevel_GEO_LEVEL_UTILITY,
		gridosv1.GeoLevel_GEO_LEVEL_SUBSTATION,
		gridosv1.GeoLevel_GEO_LEVEL_FEEDER,
	}
	path := make([]*gridosv1.GeoNode, 0, len(ids))
	parent := ""
	for index, id := range ids {
		path = append(path, &gridosv1.GeoNode{Id: id, ParentId: parent, Level: levels[index], Label: "SIMULATED " + id, Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED})
		parent = id
	}
	return path, nil
}
