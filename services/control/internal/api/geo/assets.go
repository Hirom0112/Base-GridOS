package geo

import (
	"io/fs"
	"net/http"
	"strings"
)

func AssetHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch request.URL.Path {
		case "/geo/style.json", "/geo/texas.geojson", "/geo/weather-zones.geojson", "/geo/load-zones.geojson":
			http.ServeFileFS(response, request, assets, strings.TrimPrefix(request.URL.Path, "/geo/"))
		default:
			http.NotFound(response, request)
		}
	})
}
