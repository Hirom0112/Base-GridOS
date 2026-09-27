package fleet

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLoadRetainsForecastIdentifiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleet.jsonl")
	contents := `{"device_id":"device-1","site_id":"site-1","h3_cell":"874898c80ffffff","usable_energy_kwh":40,"max_discharge_kw":5,"hardware_floor_percent":10,"load_profile_type":"RESHIWR_SCENT","reliability_trait":"HIGH","weather_zone":"SCENT","load_zone":"LZ_AEN"}`
	require.NoError(t, os.WriteFile(path, []byte(contents+"\n"), 0o600))
	sites, _, err := Load(path, NewTwin(time.Minute), time.Now())
	require.NoError(t, err)
	require.Equal(t, "RESHIWR_SCENT", sites[0].GetSite().GetLoadProfileType())
	require.Equal(t, "HIGH", sites[0].GetSite().GetReliabilityTrait())
	require.Nil(t, sites[0].GetSite().County)
}

func TestLoadKeepsHardwareFloorSeparateFromMemberPreference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleet.jsonl")
	contents := `{"device_id":"device-1","site_id":"site-1","h3_cell":"874898c80ffffff","usable_energy_kwh":40,"max_discharge_kw":5,"reserve_preference_percent":0,"hardware_floor_percent":10}`
	require.NoError(t, os.WriteFile(path, []byte(contents+"\n"), 0o600))
	now := time.Now().UTC()
	twin := NewTwin(time.Minute)
	_, adapter, err := Load(path, twin, now)
	require.NoError(t, err)
	adapter.Accept(&gridosv1.TelemetryObservation{DeviceId: "device-1", ObservationTime: timestamppb.New(now), StateOfEnergyPercent: 50,
		OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now)}}})
	state, found := twin.Site("site-1", now)
	require.True(t, found)
	require.Equal(t, 0.0, state.ReserveKWh)
	require.Equal(t, 4.0, state.HardwareFloorKWh)
}
