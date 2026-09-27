package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/jackc/pgx/v5"
)

type riskEvidence struct {
	Weather   *RiskWeather
	Outage    *RiskOutage
	Telemetry []RiskTelemetry
	Gateways  []RiskGateway
	Missing   []string
}

func (bridge *RiskBridge) Evaluate(ctx context.Context, at time.Time) error {
	if at.IsZero() {
		return errors.New("risk evaluation time required")
	}
	policy, err := bridge.store.RiskPolicyAt(ctx, at)
	if err != nil {
		return err
	}
	source, err := bridge.loadSources(ctx, at)
	if err != nil {
		return err
	}
	for _, site := range bridge.sites {
		if err := bridge.evaluateSite(ctx, at, policy, source, site); err != nil {
			return err
		}
	}
	return nil
}

func (bridge *RiskBridge) evaluateSite(ctx context.Context, at time.Time, policy RiskPolicy, source riskSource, site *gridosv1.AuthorizedSite) error {
	siteID := site.GetSite().GetSiteId()
	memberID := source.members[siteID]
	evidence, decisions := riskForSite(at, policy, source, site, bridge.fleetFile)
	if memberID == "" {
		evidence.Missing = append(evidence.Missing, "member_binding_unavailable")
	}
	var policyVersion string
	if memberID != "" && len(decisions) > 0 {
		plan, err := bridge.store.Current(ctx, memberID, at)
		if err != nil {
			return err
		}
		if plan != nil {
			policyVersion = plan.PolicyVersion
		}
		if policyVersion == "" {
			evidence.Missing = append(evidence.Missing, "consented_plan_unavailable")
		}
	}
	decisions, err := bridge.recordEvaluation(ctx, siteID, memberID, at, policy.Version, evidence, decisions)
	if err != nil || policyVersion == "" {
		return err
	}
	slices.SortStableFunc(decisions, func(left, right RiskDecision) int {
		if left.FloorPercent < right.FloorPercent {
			return -1
		}
		if left.FloorPercent > right.FloorPercent {
			return 1
		}
		return strings.Compare(string(left.Reason), string(right.Reason))
	})
	for _, decision := range decisions {
		id := fmt.Sprintf("risk:%s:%s:%s:%d", policy.Version, siteID, decision.Reason, at.UnixNano())
		if err := bridge.applyRiskDecision(ctx, id, memberID, policyVersion, at, decision); err != nil {
			return err
		}
	}
	return nil
}

func (bridge *RiskBridge) applyRiskDecision(ctx context.Context, id, memberID, policyVersion string, at time.Time, decision RiskDecision) error {
	start, err := bridge.riskCoverageLapse(ctx, memberID, decision.FloorPercent, at, decision.ExpiresAt)
	if err != nil || !start.Before(decision.ExpiresAt) {
		return err
	}
	state, err := bridge.store.ReserveAt(ctx, memberID, start)
	if err != nil || decision.FloorPercent <= state.EffectivePercent {
		return err
	}
	return bridge.store.ApplyOverride(ctx, ReserveOverride{ID: id, MemberID: memberID, Reason: decision.Reason,
		FloorPercent: decision.FloorPercent, EffectiveAt: start, ExpiresAt: decision.ExpiresAt,
		PolicyVersion: policyVersion, EvidenceID: decision.EvidenceID, CorrelationID: id})
}

func (bridge *RiskBridge) riskCoverageLapse(ctx context.Context, memberID string, floor float64, at, until time.Time) (time.Time, error) {
	lapse := at
	for lapse.Before(until) {
		var coveredUntil *time.Time
		err := bridge.pool.QueryRow(ctx, `SELECT max(expires_at) FROM reserve_overrides
			WHERE member_id = $1 AND reserve_floor_percent >= $2 AND effective_at <= $3 AND expires_at > $3`,
			memberID, floor, lapse).Scan(&coveredUntil)
		if err != nil || coveredUntil == nil {
			return lapse, err
		}
		lapse = *coveredUntil
	}
	return lapse, nil
}

func (bridge *RiskBridge) recordEvaluation(ctx context.Context, siteID, memberID string, at time.Time, policyVersion string, evidence riskEvidence, decisions []RiskDecision) ([]RiskDecision, error) {
	signalsJSON, err := json.Marshal(evidence)
	if err != nil {
		return nil, err
	}
	decisionsJSON, err := json.Marshal(decisions)
	if err != nil {
		return nil, err
	}
	var stored []byte
	err = bridge.pool.QueryRow(ctx, `INSERT INTO risk_policy_evaluations(site_id,evaluated_at,member_id,policy_version,signals,decisions)
		VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING RETURNING decisions`,
		siteID, at, memberID, policyVersion, signalsJSON, decisionsJSON).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		err = bridge.pool.QueryRow(ctx, `SELECT decisions FROM risk_policy_evaluations
			WHERE site_id = $1 AND evaluated_at = $2 AND policy_version = $3`, siteID, at, policyVersion).Scan(&stored)
	}
	if err != nil {
		return nil, err
	}
	var frozen []RiskDecision
	if err := json.Unmarshal(stored, &frozen); err != nil {
		return nil, err
	}
	return frozen, nil
}

func riskForSite(at time.Time, policy RiskPolicy, source riskSource, site *gridosv1.AuthorizedSite, fleetFile string) (riskEvidence, []RiskDecision) {
	evidence := riskEvidence{}
	if source.publicError != "" {
		evidence.Missing = append(evidence.Missing, source.publicError)
	}
	var weatherGap string
	evidence.Weather, weatherGap = weatherForSite(at, policy, source, site, fleetFile)
	if weatherGap != "" {
		evidence.Missing = append(evidence.Missing, weatherGap)
	}
	if county := site.GetSite().GetCounty(); county != "" && source.publicError == "" {
		for _, rate := range source.public.OutageRates {
			if strings.EqualFold(rate.County, county) {
				hours := float64(time.Date(at.Year(), at.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day() * 24)
				evidence.Outage = &RiskOutage{EvidenceID: rate.County + ":" + rate.Month,
					AsOf: rate.Source.AsOf, HourlyProbability: 1 - math.Exp(-rate.Rate/hours)}
				break
			}
		}
	}
	if evidence.Outage == nil {
		evidence.Missing = append(evidence.Missing, "outage_county_rate_unavailable")
	}
	decisions := policy.Evaluate(RiskSignals{At: at, Weather: evidence.Weather, Outage: evidence.Outage})
	seen := make(map[OverrideReason]bool)
	for _, decision := range decisions {
		seen[decision.Reason] = true
	}
	for _, device := range site.GetDevices() {
		deviceID := device.GetDeviceId()
		var telemetry *RiskTelemetry
		if observation := source.observations[deviceID]; observation != nil {
			_, overcurrent := observation.GetOperatingState().(*gridosv1.TelemetryObservation_OffGridOvercurrent)
			_, standby := observation.GetOperatingState().(*gridosv1.TelemetryObservation_OffGridOvercurrentStandby)
			telemetry = &RiskTelemetry{EvidenceID: observation.GetObservationId(),
				ObservedAt: observation.GetObservationTime().AsTime(), Alarm: overcurrent || standby}
			evidence.Telemetry = append(evidence.Telemetry, *telemetry)
		} else {
			evidence.Missing = append(evidence.Missing, "telemetry_unavailable:"+deviceID)
		}
		var gateway *RiskGateway
		if known, found := source.gateways[deviceID]; found {
			gateway = &known
			evidence.Gateways = append(evidence.Gateways, known)
		} else {
			evidence.Missing = append(evidence.Missing, "gateway_unavailable:"+deviceID)
		}
		for _, decision := range policy.Evaluate(RiskSignals{At: at, Telemetry: telemetry, Gateway: gateway}) {
			if !seen[decision.Reason] {
				decisions = append(decisions, decision)
				seen[decision.Reason] = true
			}
		}
	}
	return evidence, decisions
}
