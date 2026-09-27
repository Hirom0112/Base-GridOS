package stepup

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSignStepUpAssertionBindsClaimsAndUsesFreshNonce(t *testing.T) {
	now := time.Now().UTC()
	first, err := SignStepUpAssertion(LocalStepUpKey, "approver", "APPROVE_EVENT", "event-1", 3, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SignStepUpAssertion(LocalStepUpKey, "approver", "APPROVE_EVENT", "event-1", 3, now)
	if err != nil || first == second {
		t.Fatalf("repeated assertion = %q, %v", second, err)
	}
	encoded, signature, found := strings.Cut(first, ".")
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
	mac := hmac.New(sha256.New, []byte(LocalStepUpKey))
	_, _ = mac.Write(payload)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		t.Fatal("invalid signature")
	}
	var assertion stepUpAssertion
	if err := json.Unmarshal(payload, &assertion); err != nil {
		t.Fatal(err)
	}
	if assertion.Subject != "approver" || assertion.Action != "APPROVE_EVENT" || assertion.EventID != "event-1" || assertion.PlanVersion != 3 || assertion.Nonce == "" || !assertion.IssuedAt.Equal(now) || !assertion.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("unbound assertion: %+v", assertion)
	}
}

func TestStepUpCanonicalVector(t *testing.T) {
	issued := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	claims, err := json.Marshal(stepUpAssertion{Subject: "approver", Action: "APPROVE_EVENT", EventID: "event-1", PlanVersion: 3,
		IssuedAt: issued, ExpiresAt: issued.Add(5 * time.Minute), Nonce: "nonce-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"subject":"approver","action":"APPROVE_EVENT","event_id":"event-1","plan_version":3,"issued_at":"2026-09-26T12:00:00Z","expires_at":"2026-09-26T12:05:00Z","nonce":"nonce-1"}`
	if string(claims) != want {
		t.Fatalf("canonical claims = %s", claims)
	}
	mac := hmac.New(sha256.New, []byte(LocalStepUpKey))
	_, _ = mac.Write(claims)
	if signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); signature != "F3gRWWCEx-gjJTB3QMYli6IzM5tKHO9RdG-LWCD414I" {
		t.Fatalf("signature = %s", signature)
	}
}
