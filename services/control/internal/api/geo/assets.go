package geo

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

func AssetHandler(assets fs.FS) (http.Handler, error) {
	files := make(map[string][]byte, 4)
	for _, name := range []string{"style.json", "texas.geojson", "weather-zones.geojson", "load-zones.geojson"} {
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			return nil, err
		}
		if err := ValidateResponse(data); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		files[name] = data
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(request.URL.Path, "/geo/")
		data, found := files[name]
		if !found || request.URL.Path != "/geo/"+name {
			http.NotFound(response, request)
			return
		}
		http.ServeContent(response, request, name, time.Time{}, bytes.NewReader(data))
	}), nil
}
