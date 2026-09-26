package fleet

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRetainsForecastIdentifiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleet.jsonl")
	contents := `{"device_id":"device-1","site_id":"site-1","h3_cell":"874898c80ffffff","usable_energy_kwh":40,"max_discharge_kw":5,"load_profile_type":"RESHIWR_SCENT","reliability_trait":"HIGH","weather_zone":"SCENT","load_zone":"LZ_AEN"}`
	require.NoError(t, os.WriteFile(path, []byte(contents+"\n"), 0o600))
	sites, _, err := Load(path, NewTwin(time.Minute), time.Now())
	require.NoError(t, err)
	require.Equal(t, "RESHIWR_SCENT", sites[0].GetSite().GetLoadProfileType())
	require.Equal(t, "HIGH", sites[0].GetSite().GetReliabilityTrait())
	require.Nil(t, sites[0].GetSite().County)
}
