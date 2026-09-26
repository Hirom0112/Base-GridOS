package geo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	fleetgeo "github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/geo"
)

func TestNoAddress(t *testing.T) {
	root := "../../../../../testdata/fixtures/geo"
	assets := make(fstest.MapFS)
	for _, name := range []string{"style.json", "texas.geojson", "weather-zones.geojson", "load-zones.geojson"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateResponse(data); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assets[name] = &fstest.MapFile{Data: data}
	}
	cell, err := json.Marshal(fleetgeo.Cell{Cell: "87489e346ffffff", SiteCount: 5})
	if err != nil || ValidateResponse(cell) != nil {
		t.Fatalf("aggregate response rejected: %v", err)
	}
	for _, unsafe := range []string{`{"properties":{"street_address":"12 Main Street"}}`, `{"properties":{"member_home_location":[-97.7,30.2]}}`, `{"properties":{"\u0061ddress":"12 Main Street"}}`} {
		if ValidateResponse([]byte(unsafe)) == nil {
			t.Fatalf("private field accepted: %s", unsafe)
		}
	}
	assets["style.json"] = &fstest.MapFile{Data: []byte(`{"street_address":"12 Main Street"}`)}
	if _, err := AssetHandler(assets); err == nil {
		t.Fatal("unsafe geo asset served")
	}
}
