package policy

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type OfferKind string

const (
	PlanOffer       OfferKind = "PLAN"
	TravelFlexOffer OfferKind = "TRAVEL_FLEX"
)

type Offer struct {
	ID                        string
	MemberID                  string
	Kind                      OfferKind
	Market                    string
	CatalogVersion            string
	MemberPlanID              string
	ContractVersion           string
	PriceText                 string
	ConsentText               string
	ConsentVersion            string
	EffectiveAt               time.Time
	ExpiresAt                 time.Time
	TemporaryReservePercent   *float64
	CreditType                CreditType
	CreditCents               int64
	EnergyMonthlyChargeCents  int64
	BatteryMonthlyChargeCents int64
	FlexibilityRewardCents    int64
	CorrelationID             string
}

func (offer Offer) validate() error {
	if err := offer.validateIdentity(); err != nil {
		return err
	}
	switch offer.Kind {
	case PlanOffer:
		if offer.TemporaryReservePercent != nil || offer.CreditType != "" || offer.CreditCents != 0 {
			return errors.New("plan offer cannot carry Travel Flex terms")
		}
	case TravelFlexOffer:
		return offer.validateFlexTerms()
	default:
		return errors.New("unsupported offer type")
	}
	return nil
}

func (offer Offer) validateIdentity() error {
	if offer.ID == "" || offer.MemberID == "" || offer.Market == "" || offer.CatalogVersion == "" || offer.MemberPlanID == "" || offer.ContractVersion == "" || offer.PriceText == "" || offer.ConsentText == "" || offer.ConsentVersion == "" || offer.CorrelationID == "" || offer.EffectiveAt.IsZero() || !offer.ExpiresAt.After(offer.EffectiveAt) {
		return errors.New("offer identity, presented terms, catalog, and bounded interval are required")
	}
	return nil
}

func (offer Offer) validateFlexTerms() error {
	if offer.TemporaryReservePercent == nil || math.IsNaN(*offer.TemporaryReservePercent) || math.IsInf(*offer.TemporaryReservePercent, 0) || *offer.TemporaryReservePercent < 0 || *offer.TemporaryReservePercent > 100 || offer.CreditCents < 0 {
		return errors.New("travel flex reserve and fixed credit are required")
	}
	switch offer.CreditType {
	case "FIXED_DAILY", "FIXED_EVENT", "FIXED_ANNUAL":
		return nil
	default:
		return errors.New("travel flex credit type must be fixed")
	}
}

func (store *Store) PresentOffer(ctx context.Context, offer Offer) (*Offer, error) {
	if err := offer.validate(); err != nil {
		return nil, err
	}
	var catalogReward int64
	err := store.pool.QueryRow(ctx, `SELECT energy_monthly_charge_cents, battery_monthly_charge_cents, flexibility_reward_cents
		FROM pricing_catalog_snapshots WHERE catalog_version = $1 AND member_plan_id = $2 AND market = $3
		AND effective_at <= $4 AND (expires_at IS NULL OR expires_at > $4)`,
		offer.CatalogVersion, offer.MemberPlanID, offer.Market, offer.EffectiveAt).Scan(
		&offer.EnergyMonthlyChargeCents, &offer.BatteryMonthlyChargeCents, &catalogReward)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("offer catalog entry is not effective")
	}
	if err != nil {
		return nil, err
	}
	offer.FlexibilityRewardCents = catalogReward
	if offer.Kind == TravelFlexOffer {
		offer.FlexibilityRewardCents = offer.CreditCents
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "offer:"+offer.ID); err != nil {
		return nil, err
	}
	previous, err := presentedOffer(ctx, tx, offer.ID)
	if err != nil {
		return nil, err
	}
	if previous != nil {
		if !sameOffer(*previous, offer) {
			return nil, errors.New("offer idempotency key has different presented terms")
		}
		return previous, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO flexibility_offers
		(offer_id, member_id, catalog_version, contract_version, flexibility_reward_cents, price_text,
		effective_at, expires_at, correlation_id, member_plan_id, market, offer_type, consent_text, consent_version,
		temporary_reserve_percent, credit_type, energy_monthly_charge_cents, battery_monthly_charge_cents)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		offer.ID, offer.MemberID, offer.CatalogVersion, offer.ContractVersion, offer.FlexibilityRewardCents,
		offer.PriceText, offer.EffectiveAt, offer.ExpiresAt, offer.CorrelationID, offer.MemberPlanID, offer.Market, offer.Kind,
		offer.ConsentText, offer.ConsentVersion, offer.TemporaryReservePercent, nullableCreditType(offer),
		offer.EnergyMonthlyChargeCents, offer.BatteryMonthlyChargeCents)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_journal(actor_id, action, resource_type, resource_id, new_values, correlation_id)
		VALUES ($1, 'FLEXIBILITY_OFFER_PRESENTED', 'flexibility_offer', $2,
		jsonb_build_object('catalog_version', $3::text, 'member_plan_id', $4::text, 'contract_version', $5::text,
		'consent_version', $6::text, 'price_text', $7::text), $8)`, offer.MemberID, offer.ID,
		offer.CatalogVersion, offer.MemberPlanID, offer.ContractVersion, offer.ConsentVersion, offer.PriceText, offer.CorrelationID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &offer, nil
}

func nullableCreditType(offer Offer) *string {
	if offer.Kind != TravelFlexOffer {
		return nil
	}
	value := string(offer.CreditType)
	return &value
}

func presentedOffer(ctx context.Context, tx pgx.Tx, id string) (*Offer, error) {
	var offer Offer
	var creditType *string
	err := tx.QueryRow(ctx, `SELECT offer_id, member_id, offer_type, catalog_version, member_plan_id, market,
		contract_version, price_text, consent_text, consent_version, effective_at, expires_at,
		temporary_reserve_percent, credit_type, energy_monthly_charge_cents,
		battery_monthly_charge_cents, flexibility_reward_cents, correlation_id
		FROM flexibility_offers WHERE offer_id = $1`, id).Scan(&offer.ID, &offer.MemberID, &offer.Kind,
		&offer.CatalogVersion, &offer.MemberPlanID, &offer.Market, &offer.ContractVersion, &offer.PriceText,
		&offer.ConsentText, &offer.ConsentVersion, &offer.EffectiveAt, &offer.ExpiresAt,
		&offer.TemporaryReservePercent, &creditType, &offer.EnergyMonthlyChargeCents,
		&offer.BatteryMonthlyChargeCents, &offer.FlexibilityRewardCents, &offer.CorrelationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if creditType != nil {
		offer.CreditType = CreditType(*creditType)
		offer.CreditCents = offer.FlexibilityRewardCents
	}
	return &offer, nil
}

func sameOffer(previous, requested Offer) bool {
	if previous.TemporaryReservePercent == nil && requested.TemporaryReservePercent != nil || previous.TemporaryReservePercent != nil && requested.TemporaryReservePercent == nil {
		return false
	}
	if previous.TemporaryReservePercent != nil && *previous.TemporaryReservePercent != *requested.TemporaryReservePercent {
		return false
	}
	if !previous.EffectiveAt.Equal(requested.EffectiveAt) || !previous.ExpiresAt.Equal(requested.ExpiresAt) {
		return false
	}
	previous.EffectiveAt = requested.EffectiveAt
	previous.ExpiresAt = requested.ExpiresAt
	previous.TemporaryReservePercent = requested.TemporaryReservePercent
	return previous == requested
}
