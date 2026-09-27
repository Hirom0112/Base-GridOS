package main

import (
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
)

func TestFleetSeedRejectsMixedProvenance(t *testing.T) {
	first := int64(42)
	second := int64(43)
	sites := []*gridosv1.AuthorizedSite{
		{Site: &gridosv1.Site{Provenance: &gridosv1.Provenance{SimulationSeed: &first}}},
		{Site: &gridosv1.Site{Provenance: &gridosv1.Provenance{SimulationSeed: &second}}},
	}
	_, err := fleetSeed(sites)
	require.ErrorContains(t, err, "mixed simulation seeds")
	sites[1].Site.Provenance.SimulationSeed = &first
	seed, err := fleetSeed(sites)
	require.NoError(t, err)
	require.Equal(t, first, seed)
}
