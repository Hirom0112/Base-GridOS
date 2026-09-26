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
	index := []byte(`{"screens":{"fleet":["gridos.v1.FleetService.GetFleetSummary","gridos.v1.FleetService.ListSites"],"dispatch":["gridos.v1.DispatchService.CreateEventRequest","gridos.v1.DispatchService.GetEvent","gridos.v1.DispatchService.ApproveEvent"]},"requests":{"gridos.v1.FleetService.GetFleetSummary":{},"gridos.v1.FleetService.ListSites":{},"gridos.v1.DispatchService.CreateEventRequest":{"idempotencyKey":"idem"},"gridos.v1.DispatchService.GetEvent":{"eventId":"event"},"gridos.v1.DispatchService.ApproveEvent":{"eventId":"event"}}}`)
	if err := os.WriteFile(filepath.Join(fixtureRoot, "INDEX.json"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	var mutex sync.Mutex
	called := make(map[string]int)
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
	if len(called) != 5 {
		t.Fatalf("called %d methods, want 5", len(called))
	}
	for path, count := range called {
		if count != 1 {
			t.Errorf("%s called %d times", path, count)
		}
	}
	for service, methods := range map[string][]string{
		"FleetService":    {"GetFleetSummary", "ListSites"},
		"DispatchService": {"CreateEventRequest", "GetEvent", "ApproveEvent"},
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
