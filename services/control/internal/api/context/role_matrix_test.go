package context

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
)

func TestRoleMatrixContext(t *testing.T) {
	_, handler := gridosv1connect.NewContextServiceHandler(NewService("", time.Now))
	server := httptest.NewServer(handler)
	defer server.Close()
	roles := []string{"operator", "approver", "analyst", "partner", "service", "member", "", "unknown"}
	cases := []struct {
		method string
		body   string
	}{
		{"GetMarketContext", `{"settlementPoint":"HB_HOUSTON","weatherZone":"NORTH_C"}`},
		{"GetWeatherContext", `{"city":"austin"}`},
		{"GetOutageRisk", `{"county":"Travis"}`},
		{"ListDispatchWindows", `{}`},
	}
	for _, entry := range cases {
		for _, role := range roles {
			request, err := http.NewRequest(http.MethodPost, server.URL+"/gridos.v1.ContextService/"+entry.method, strings.NewReader(entry.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-GridOS-Role", role)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			allowed := strings.Contains(" operator approver analyst partner service ", " "+role+" ") && role != ""
			if allowed && response.StatusCode == http.StatusForbidden {
				t.Errorf("%s %q rejected", entry.method, role)
			}
			if role == "" && response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s missing role status = %d", entry.method, response.StatusCode)
			}
			if !allowed && role != "" && response.StatusCode != http.StatusForbidden {
				t.Errorf("%s %q status = %d, want 403", entry.method, role, response.StatusCode)
			}
		}
	}
}
