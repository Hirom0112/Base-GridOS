package api

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRoleMatrixControl(t *testing.T) {
	server, _, _, _ := testServer(t)
	defer server.Close()
	roles := []string{"operator", "approver", "analyst", "partner", "service", "member", "", "unknown"}
	cases := []struct {
		method  string
		allowed string
	}{
		{"FleetService/GetFleetSummary", "operator approver analyst partner service"},
		{"FleetService/ListSites", "operator approver analyst partner service"},
		{"DispatchService/CreateEventRequest", "operator"},
		{"DispatchService/GetEvent", "operator approver analyst partner service"},
		{"DispatchService/ApproveEvent", "approver"},
		{"DispatchService/LaunchEvent", "approver"},
		{"DispatchService/GetPlanExplanation", "operator approver analyst service"},
		{"DispatchService/ValidateUnsafeAlternative", "operator approver"},
	}
	for _, entry := range cases {
		for _, role := range roles {
			request, err := http.NewRequest(http.MethodPost, server.URL+"/gridos.v1."+entry.method, strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(roleHeader, role)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			allowed := strings.Contains(" "+entry.allowed+" ", " "+role+" ") && role != ""
			if role == "" && response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s missing role status = %d, want 401", entry.method, response.StatusCode)
				continue
			}
			if role == "" {
				continue
			}
			if allowed && response.StatusCode == http.StatusForbidden {
				t.Errorf("%s %q rejected by role gate", entry.method, role)
			}
			if !allowed && response.StatusCode != http.StatusForbidden {
				t.Errorf("%s %q status = %d, want 403", entry.method, role, response.StatusCode)
			}
		}
	}
}
