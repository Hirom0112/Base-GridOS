package geo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOfflineAssets(t *testing.T) {
	handler := AssetHandler(os.DirFS("../../../../../testdata/fixtures/geo"))
	for _, path := range []string{"style.json", "texas.geojson", "weather-zones.geojson", "load-zones.geojson"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/geo/"+path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, response.Code)
		}
		body := response.Body.Bytes()
		if !json.Valid(body) {
			t.Fatalf("%s: invalid JSON", path)
		}
		if path == "style.json" && (strings.Contains(string(body), "http:") || strings.Contains(string(body), "https:")) {
			t.Fatal("style requires network assets")
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/geo/private.json", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected asset status %d", response.Code)
	}
}
