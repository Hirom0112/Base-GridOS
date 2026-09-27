package stepup

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const LocalStepUpKey = "gridos-local-step-up-key-32-bytes-minimum"

type stepUpAssertion struct {
	Subject     string    `json:"subject"`
	Action      string    `json:"action"`
	EventID     string    `json:"event_id"`
	PlanVersion uint64    `json:"plan_version"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Nonce       string    `json:"nonce"`
}

func SignStepUpAssertion(key, subject, action, eventID string, planVersion uint64, now time.Time) (string, error) {
	if len(key) < 32 || subject == "" || eventID == "" {
		return "", errors.New("step-up signing inputs are incomplete")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	claims, err := json.Marshal(stepUpAssertion{Subject: subject, Action: action, EventID: eventID, PlanVersion: planVersion,
		IssuedAt: now.UTC(), ExpiresAt: now.UTC().Add(5 * time.Minute), Nonce: hex.EncodeToString(nonce[:])})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(claims)
	return base64.RawURLEncoding.EncodeToString(claims) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
