package stepup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Assertion struct {
	Subject     string    `json:"subject"`
	Action      string    `json:"action"`
	EventID     string    `json:"event_id"`
	PlanVersion uint64    `json:"plan_version"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Nonce       string    `json:"nonce"`
}

type Verifier struct {
	pool *pgxpool.Pool
	key  []byte
	now  func() time.Time
}

func New(pool *pgxpool.Pool, key []byte, now func() time.Time) *Verifier {
	return &Verifier{pool: pool, key: bytes.Clone(key), now: now}
}

func FromEnvironment(pool *pgxpool.Pool, now func() time.Time) *Verifier {
	key := os.Getenv("GRIDOS_STEP_UP_KEY")
	if key == "" {
		return nil
	}
	return New(pool, []byte(key), now)
}

func (verifier *Verifier) Verify(ctx context.Context, token, action, eventID string, planVersion uint64) (string, error) {
	if verifier == nil || verifier.pool == nil || len(verifier.key) < 32 || len(token) > 4096 {
		return "", errors.New("step-up verifier is unavailable")
	}
	assertion, canonical, err := parseAssertion(token, verifier.key)
	if err != nil {
		return "", err
	}
	now := verifier.now()
	if err = validateAssertion(assertion, action, eventID, planVersion, now); err != nil {
		return "", err
	}
	if err = verifier.accept(ctx, assertion, canonical, now); err != nil {
		return "", err
	}
	return assertion.Subject, nil
}

func parseAssertion(token string, key []byte) (Assertion, []byte, error) {
	encoded, signature, found := strings.Cut(token, ".")
	if !found || strings.Contains(signature, ".") {
		return Assertion{}, nil, errors.New("invalid step-up assertion")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) == 0 {
		return Assertion{}, nil, errors.New("invalid step-up assertion")
	}
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || len(provided) != sha256.Size {
		return Assertion{}, nil, errors.New("invalid step-up signature")
	}
	var assertion Assertion
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&assertion); err != nil {
		return Assertion{}, nil, errors.New("invalid step-up claims")
	}
	var trailing json.RawMessage
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Assertion{}, nil, errors.New("invalid step-up claims")
	}
	canonical, err := json.Marshal(assertion)
	if err != nil {
		return Assertion{}, nil, err
	}
	if !bytes.Equal(payload, canonical) {
		return Assertion{}, nil, errors.New("noncanonical step-up claims")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(canonical)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return Assertion{}, nil, errors.New("invalid step-up signature")
	}
	return assertion, canonical, nil
}

func validateAssertion(assertion Assertion, action, eventID string, planVersion uint64, now time.Time) error {
	if assertion.Subject == "" || assertion.Nonce == "" || assertion.Action != action || assertion.EventID != eventID || assertion.PlanVersion != planVersion || assertion.IssuedAt.After(now) || !assertion.ExpiresAt.After(now) || !assertion.ExpiresAt.After(assertion.IssuedAt) || assertion.ExpiresAt.Sub(assertion.IssuedAt) > 5*time.Minute {
		return errors.New("step-up assertion is expired or unbound")
	}
	return nil
}

func (verifier *Verifier) accept(ctx context.Context, assertion Assertion, canonical []byte, now time.Time) error {
	tx, err := verifier.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `INSERT INTO step_up_assertions(nonce,subject,action,event_id,plan_version,issued_at,expires_at,accepted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (nonce) DO NOTHING`, assertion.Nonce,
		assertion.Subject, assertion.Action, assertion.EventID, assertion.PlanVersion, assertion.IssuedAt, assertion.ExpiresAt, now)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return errors.New("step-up assertion was already used")
	}
	if err = storage.AppendAudit(ctx, tx, storage.AuditRecord{OccurredAt: now, ActorID: assertion.Subject,
		Action: "STEP_UP_ACCEPTED", ResourceID: assertion.EventID, NewValues: canonical, CorrelationID: assertion.EventID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
