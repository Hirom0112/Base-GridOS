package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRecordWritesEveryIndexedMethod(t *testing.T) {
	fixtureRoot := t.TempDir()
	index := []byte(`{"screens":{"fleet":["gridos.v1.FleetService.GetFleetSummary","gridos.v1.FleetService.ListSites"],"dispatch":["gridos.v1.DispatchService.CreateEventRequest","gridos.v1.DispatchService.GetEvent","gridos.v1.DispatchService.ApproveEvent","gridos.v1.DispatchService.LaunchEvent"]},"requests":{"gridos.v1.FleetService.GetFleetSummary":{"role":"operator","body":{}},"gridos.v1.FleetService.ListSites":{"role":"operator","body":{}},"gridos.v1.DispatchService.CreateEventRequest":{"role":"operator","body":{"idempotencyKey":"idem"}},"gridos.v1.DispatchService.GetEvent":{"role":"operator","body":{"eventId":"event"}},"gridos.v1.DispatchService.ApproveEvent":{"role":"approver","body":{"eventId":"event"}},"gridos.v1.DispatchService.LaunchEvent":{"role":"approver","body":{"eventId":"event"}}}}`)
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
