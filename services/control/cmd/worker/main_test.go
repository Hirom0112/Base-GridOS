package main

import (
	"runtime/debug"
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

func TestCodeVersionPrefersRevisionAndRejectsUnknownBuild(t *testing.T) {
	build := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}, {Key: "vcs.modified", Value: "true"}}}
	version, err := codeVersion(build, "override")
	require.NoError(t, err)
	require.Equal(t, "abc123+modified", version)
	build.Settings[1].Value = "false"
	version, err = codeVersion(build, "override")
	require.NoError(t, err)
	require.Equal(t, "abc123", version)
	version, err = codeVersion(&debug.BuildInfo{}, "override-123")
	require.NoError(t, err)
	require.Equal(t, "override-123", version)
	_, err = codeVersion(&debug.BuildInfo{}, "")
	require.ErrorContains(t, err, "code version")
}
