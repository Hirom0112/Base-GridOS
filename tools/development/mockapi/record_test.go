package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecordSignsEmergencyStopWithoutPlanVersion(t *testing.T) {
	t.Setenv("GRIDOS_STEP_UP_KEY", "gridos-local-step-up-key-32-bytes-minimum")
	approval, err := recordStepUpAssertion("gridos.v1.DispatchService.ApproveEvent", []byte(`{"eventId":"event-1","planVersion":"1","approvedBy":"approver"}`))
	if err != nil || approval == "" {
		t.Fatalf("approval assertion = %q, %v", approval, err)
	}
	stop, err := recordStepUpAssertion("gridos.v1.DispatchService.EmergencyStop", []byte(`{"eventId":"event-1","requestedBy":"operator"}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, found := strings.Cut(stop, ".")
	if !found {
		t.Fatal("missing signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var assertion struct {
		Subject     string `json:"subject"`
		Action      string `json:"action"`
		EventID     string `json:"event_id"`
		PlanVersion uint64 `json:"plan_version"`
	}
	if err := json.Unmarshal(payload, &assertion); err != nil {
		t.Fatal(err)
	}
	if assertion.Subject != "operator" || assertion.Action != "EMERGENCY_STOP" || assertion.EventID != "event-1" || assertion.PlanVersion != 0 {
		t.Fatalf("stop assertion = %+v", assertion)
	}
}

func TestRecordWritesEveryIndexedMethod(t *testing.T) {
	fixtureRoot := t.TempDir()
	index := []byte(`{"screens":{"fleet":["gridos.v1.FleetService.GetFleetSummary","gridos.v1.FleetService.ListSites"],"dispatch":["gridos.v1.DispatchService.CreateEventRequest","gridos.v1.DispatchService.GetEvent","gridos.v1.DispatchService.ApproveEvent","gridos.v1.DispatchService.LaunchEvent"]},"requests":{"gridos.v1.FleetService.GetFleetSummary":{"role":"operator","body":{}},"gridos.v1.FleetService.ListSites":{"role":"operator","body":{}},"gridos.v1.DispatchService.CreateEventRequest":{"role":"operator","body":{"idempotencyKey":"idem"}},"gridos.v1.DispatchService.GetEvent":{"role":"operator","body":{"eventId":"event"}},"gridos.v1.DispatchService.ApproveEvent":{"role":"approver","body":{"eventId":"event","planVersion":"1","approvedBy":"approver"}},"gridos.v1.DispatchService.LaunchEvent":{"role":"approver","body":{"eventId":"event"}}}}`)
	if err := os.WriteFile(filepath.Join(fixtureRoot, "INDEX.json"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	var mutex sync.Mutex
	called := make(map[string]int)
	var lifecycle []string
	stub := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		if len(body) == 0 {
			t.Error("empty request")
		}
		mutex.Lock()
		called[request.URL.Path]++
		if request.URL.Path != "/gridos.v1.FleetService/GetFleetSummary" && request.URL.Path != "/gridos.v1.FleetService/ListSites" {
			lifecycle = append(lifecycle, request.URL.Path+" "+request.Header.Get("X-GridOS-Role"))
		}
		mutex.Unlock()
		response.Header().Set("Content-Type", "application/json")
		if _, err := response.Write([]byte(`{"recorded":true}`)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(stub.Close)
	client := &http.Client{Timeout: time.Second}
	if err := recordFixtures(fixtureRoot, stub.URL, client); err != nil {
		t.Fatal(err)
	}
	if len(called) != 6 {
		t.Fatalf("called %d methods, want 6", len(called))
	}
	for path, count := range called {
		if count != 1 {
			t.Errorf("%s called %d times", path, count)
		}
	}
	wantLifecycle := []string{
		"/gridos.v1.DispatchService/CreateEventRequest operator",
		"/gridos.v1.DispatchService/GetEvent operator",
		"/gridos.v1.DispatchService/ApproveEvent approver",
		"/gridos.v1.DispatchService/LaunchEvent approver",
	}
	for index, want := range wantLifecycle {
		if lifecycle[index] != want {
			t.Errorf("lifecycle call %d = %q, want %q", index, lifecycle[index], want)
		}
	}
	for service, methods := range map[string][]string{
		"FleetService":    {"GetFleetSummary", "ListSites"},
		"DispatchService": {"CreateEventRequest", "GetEvent", "ApproveEvent", "LaunchEvent"},
	} {
		for _, method := range methods {
			content, err := os.ReadFile(filepath.Join(fixtureRoot, service, method+".json"))
			if err != nil {
				t.Error(err)
			}
			if string(content) != "{\"recorded\":true}\n" {
				t.Errorf("unexpected %s response: %s", method, content)
			}
		}
	}
}

func TestRecordSendsScopedMemberIdentity(t *testing.T) {
	fixtureRoot := t.TempDir()
	index := []byte(`{"screens":{"fleet":["gridos.v1.FleetService.GetFleetSummary"],"member":["gridos.v1.MemberService.GetMemberStatus"]},"requests":{"gridos.v1.FleetService.GetFleetSummary":{"role":"operator","body":{}},"gridos.v1.MemberService.GetMemberStatus":{"role":"member","memberId":"member-1","body":{"memberId":"member-1","siteId":"site-1"}}}}`)
	if err := os.WriteFile(filepath.Join(fixtureRoot, "INDEX.json"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	operatorCalled := false
	stub := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/gridos.v1.FleetService/GetFleetSummary" {
			operatorCalled = true
			if request.Header.Get("X-GridOS-Member-ID") != "" {
				t.Error("operator request carries member identity")
			}
		} else if request.Header.Get("X-GridOS-Member-ID") != "member-1" {
			http.Error(response, "member identity required", http.StatusForbidden)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"recorded":true}`))
	}))
	defer stub.Close()
	err := recordFixtures(fixtureRoot, stub.URL, &http.Client{Timeout: time.Second})
	if !operatorCalled {
		t.Fatal("operator control path was not called")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecordSendsStepUpForApproval(t *testing.T) {
	t.Setenv("GRIDOS_STEP_UP_KEY", "gridos-local-step-up-key-32-bytes-minimum")
	fixtureRoot := t.TempDir()
	index := []byte(`{"screens":{"dispatch":["gridos.v1.DispatchService.ApproveEvent"]},"requests":{"gridos.v1.DispatchService.ApproveEvent":{"role":"approver","body":{"eventId":"event-1","planVersion":1,"approvedBy":"approver"}}}}`)
	if err := os.WriteFile(filepath.Join(fixtureRoot, "INDEX.json"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	stub := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-GridOS-Step-Up") == "" {
			http.Error(response, "step-up required", http.StatusForbidden)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"recorded":true}`))
	}))
	defer stub.Close()
	if err := recordFixtures(fixtureRoot, stub.URL, &http.Client{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordSendsIndexedPermissions(t *testing.T) {
	fixtureRoot := t.TempDir()
	index := []byte(`{"screens":{"geo":["gridos.v1.GeoService.Drilldown"]},"requests":{"gridos.v1.GeoService.Drilldown":{"role":"operator","permissions":"site_location","body":{"parentId":"feeder:one"}}}}`)
	if err := os.WriteFile(filepath.Join(fixtureRoot, "INDEX.json"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	stub := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		called = true
		if request.Header.Get("X-GridOS-Permissions") != "site_location" {
			http.Error(response, "site_location permission required", http.StatusForbidden)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"recorded":true}`))
	}))
	defer stub.Close()
	err := recordFixtures(fixtureRoot, stub.URL, &http.Client{Timeout: time.Second})
	if !called {
		t.Fatal("recorder did not call the permission-protected method")
	}
	if err != nil {
		t.Fatal(err)
	}
}
