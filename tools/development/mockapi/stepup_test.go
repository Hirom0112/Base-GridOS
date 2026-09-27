package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalStepUpIssuesBoundAssertion(t *testing.T) {
	key := "gridos-local-step-up-key-32-bytes-minimum"
	t.Setenv("GRIDOS_STEP_UP_KEY", key)
	server := httptest.NewServer(server{fixtureRoot: t.TempDir()})
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/local/step-up", bytes.NewBufferString(`{"action":"APPROVE_EVENT","event_id":"event-1","plan_version":3}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GridOS-Role", "approver")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("step-up status = %d cache = %q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	var body struct {
		Assertion string `json:"assertion"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	encoded, signature, found := strings.Cut(body.Assertion, ".")
	if !found {
		t.Fatal("missing signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(payload)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		t.Fatal("invalid assertion signature")
	}
	var claims struct {
		Subject     string    `json:"subject"`
		Action      string    `json:"action"`
		EventID     string    `json:"event_id"`
		PlanVersion uint64    `json:"plan_version"`
		IssuedAt    time.Time `json:"issued_at"`
		ExpiresAt   time.Time `json:"expires_at"`
		Nonce       string    `json:"nonce"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "local-approver" || claims.Action != "APPROVE_EVENT" || claims.EventID != "event-1" || claims.PlanVersion != 3 || claims.Nonce == "" || claims.ExpiresAt.Sub(claims.IssuedAt) != 5*time.Minute {
		t.Fatalf("unbound assertion: %+v", claims)
	}
}

func TestLocalStepUpRejectsUnboundRequests(t *testing.T) {
	t.Setenv("GRIDOS_STEP_UP_KEY", "gridos-local-step-up-key-32-bytes-minimum")
	server := httptest.NewServer(server{fixtureRoot: t.TempDir()})
	defer server.Close()
	cases := []struct {
		role, body string
		status     int
	}{
		{"operator", `{"action":"APPROVE_EVENT","event_id":"event-1","plan_version":3}`, http.StatusForbidden},
		{"approver", `{"action":"APPROVE_EVENT","event_id":"event-1","plan_version":0}`, http.StatusBadRequest},
		{"operator", `{"action":"EMERGENCY_STOP","event_id":"event-1","plan_version":1}`, http.StatusBadRequest},
		{"approver", `{"action":"APPROVE_EVENT","event_id":"event-1","plan_version":3,"subject":"forged"}`, http.StatusBadRequest},
		{"", `{"action":"APPROVE_EVENT","event_id":"event-1","plan_version":3}`, http.StatusForbidden},
	}
	for _, entry := range cases {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/local/step-up", strings.NewReader(entry.body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-GridOS-Role", entry.role)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != entry.status {
			t.Errorf("role %q body %s status = %d, want %d", entry.role, entry.body, response.StatusCode, entry.status)
		}
	}
}

func TestLocalStepUpRequiresSigningKey(t *testing.T) {
	t.Setenv("GRIDOS_STEP_UP_KEY", "")
	server := httptest.NewServer(server{fixtureRoot: t.TempDir()})
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/local/step-up", strings.NewReader(`{"action":"EMERGENCY_STOP","event_id":"event-1","plan_version":0}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-GridOS-Role", "operator")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("missing key status = %d", response.StatusCode)
	}
}
