package member

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
)

func TestRoleMatrixMember(t *testing.T) {
	pool := memberDatabase(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO member_sites(site_id,member_id,bound_at,source,provenance)
		VALUES ('site-1','member-1',now(),'SIMULATED','{"provenance":"SIMULATED"}')`); err != nil {
		t.Fatal(err)
	}
	_, handler := gridosv1connect.NewMemberServiceHandler(NewService(pool, fleet.NewTwin(time.Minute), nil, time.Now))
	server := httptest.NewServer(handler)
	defer server.Close()
	roles := []string{"operator", "approver", "analyst", "partner", "service", "member", "", "unknown"}
	methods := []string{"GetMemberStatus", "PresentOffer", "SelectResiliencePlan", "ScheduleTravelFlex", "EndTravelFlexEarly", "SetAnomalyPreference", "ScheduleAway", "EndAway", "ListHomeActivityAlerts"}
	for _, method := range methods {
		for _, role := range roles {
			body := `{"memberId":"member-1"}`
			if method == "GetMemberStatus" {
				body = `{"memberId":"member-1","siteId":"site-1"}`
			}
			request, err := http.NewRequest(http.MethodPost, server.URL+"/gridos.v1.MemberService/"+method, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-GridOS-Role", role)
			if role == "member" {
				request.Header.Set("X-GridOS-Member-ID", "member-1")
			}
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			allowed := role == "member" || role == "operator" && method == "GetMemberStatus"
			if allowed && response.StatusCode == http.StatusForbidden {
				t.Errorf("%s %q rejected", method, role)
			}
			if role == "" && response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s missing role status = %d, want 401", method, response.StatusCode)
			}
			if !allowed && role != "" && response.StatusCode != http.StatusForbidden {
				t.Errorf("%s %q status = %d, want 403", method, role, response.StatusCode)
			}
		}
	}
}
