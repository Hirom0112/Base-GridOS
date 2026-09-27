package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/reconciliation"
	reporting "github.com/Hirom0112/Base-GridOS/services/control/internal/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type PostgresReportSource struct {
	pool *pgxpool.Pool
}

func NewPostgresReportSource(pool *pgxpool.Pool) *PostgresReportSource {
	return &PostgresReportSource{pool: pool}
}

func (source *PostgresReportSource) StoredReport(ctx context.Context, eventID string) (*reporting.EventReport, error) {
	return storage.LoadPublishedReport(ctx, source.pool, eventID)
}

func (source *PostgresReportSource) StoredReportVersion(ctx context.Context, eventID string, version uint64) (*reporting.EventReport, error) {
	return storage.LoadPublishedReportVersion(ctx, source.pool, eventID, version)
}

func (source *PostgresReportSource) EventReportData(ctx context.Context, eventID string) (reporting.StoredEvent, error) {
	var report reporting.StoredEvent
	var approved bool
	var planVersion int64
	var begin, end time.Time
	err := source.pool.QueryRow(ctx, `SELECT request.target_kw / 1000.0, event.plan_version,
		request.begin_time, request.end_time,
        EXISTS (SELECT 1 FROM operator_approvals WHERE event_id = event.event_id AND decision = 'APPROVED')
        FROM dispatch_events AS event JOIN dispatch_requests AS request USING (request_id)
		WHERE event.event_id = $1`, eventID).Scan(&report.RequestedMW, &planVersion, &begin, &end, &approved)
	if err != nil {
		return report, err
	}
	report.PlanVersion = uint64(planVersion)
	if approved {
		report.ApprovedMW = report.RequestedMW
	}
	err = source.pool.QueryRow(ctx, `SELECT
        COALESCE((SELECT sum(setpoint_kw) / 1000.0 FROM command_intents WHERE event_id = $1), 0),
        COALESCE((SELECT sum(intent.setpoint_kw) / 1000.0
          FROM command_intents AS intent
          JOIN command_acknowledgements AS acknowledgement USING (command_id)
          WHERE intent.event_id = $1 AND acknowledgement.receipt_status = 'ACCEPTED'), 0)`, eventID).Scan(&report.CommandedMW, &report.AcknowledgedMW)
	if err != nil {
		return report, err
	}
	var exclusions []byte
	err = source.pool.QueryRow(ctx, `SELECT exclusions, policy_version
        FROM eligibility_snapshots WHERE event_id = $1 ORDER BY captured_at DESC LIMIT 1`, eventID).Scan(&exclusions, &report.Versions.Policy)
	if err == nil {
		report.Exclusions, err = reportExclusions(exclusions)
		if err != nil {
			return report, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return report, err
	}
	err = source.pool.QueryRow(ctx, `SELECT solver_version, model_version
        FROM plan_versions WHERE event_id = $1 ORDER BY version DESC LIMIT 1`, eventID).Scan(&report.Versions.Solver, &report.Versions.Model)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return report, err
	}
	report.Provenance = []string{"SIMULATED"}
	report.Delivered, err = reconciliation.LoadDelivered(ctx, source.pool, eventID)
	if err != nil {
		return report, err
	}
	if report.Delivered != nil && finiteLiveReport(report.Delivered.DeliveredMWh) && report.Delivered.DeliveredMWh >= 0 {
		report.Energy = &reporting.EnergyTotals{DeliveredMWh: report.Delivered.DeliveredMWh}
	} else {
		addLiveReportGap(&report, begin, end, "delivered_energy_unavailable")
	}
	for _, reason := range []string{"requested_energy_unavailable", "approved_energy_unavailable", "commanded_energy_unavailable", "acknowledged_energy_unavailable", "reserve_violations_prevented_unavailable", "modeled_economics_unavailable"} {
		addLiveReportGap(&report, begin, end, reason)
	}
	var rewardCount int64
	var rewardCents int64
	err = source.pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(amount_cents), 0)::bigint
		FROM reward_ledger WHERE event_id = $1`, eventID).Scan(&rewardCount, &rewardCents)
	if err != nil {
		return report, err
	}
	if rewardCount == 0 {
		addLiveReportGap(&report, begin, end, "reward_unposted")
	} else {
		report.MemberRewardsCents = &rewardCents
		report.Provenance = append(report.Provenance, "REWARD_LEDGER")
	}
	return source.fillPlanEvidence(ctx, eventID, planVersion, begin, end, &report)
}

func (source *PostgresReportSource) fillPlanEvidence(ctx context.Context, eventID string, planVersion int64, begin, end time.Time, report *reporting.StoredEvent) (reporting.StoredEvent, error) {
	if planVersion == 0 {
		addLiveReportGap(report, begin, end, "plan_unavailable")
		addLiveReportGap(report, begin, end, "reserve_measurement_unavailable")
		fillLiveForecast(report, nil, begin, end)
		return *report, nil
	}
	var inputID, eligibilityID string
	var planJSON []byte
	err := source.pool.QueryRow(ctx, `SELECT input_snapshot_id, eligibility_snapshot_id, plan
		FROM plan_versions WHERE event_id = $1 AND version = $2`, eventID, planVersion).Scan(&inputID, &eligibilityID, &planJSON)
	if err != nil {
		return *report, err
	}
	plan := new(gridosv1.DispatchPlan)
	if err = protojson.Unmarshal(planJSON, plan); err != nil {
		return *report, err
	}
	if margin, valid := sourcedMargin(plan.GetMarginExplanation()); valid {
		report.Margin = margin
		report.Provenance = append(report.Provenance, "STORED_PLAN_MARGIN")
	} else {
		addLiveReportGap(report, begin, end, "margin_unavailable")
	}
	frozen, err := storage.NewPostgresEventStore(source.pool).LoadFrozen(ctx, eventID, inputID, eligibilityID)
	if err != nil {
		return *report, err
	}
	fillLiveForecast(report, frozen, begin, end)
	if err := source.fillReserveCompliance(ctx, begin, end, frozen, report); err != nil {
		return *report, err
	}
	return *report, nil
}

func sourcedMargin(explanation *gridosv1.MarginExplanation) (*reporting.ModeledMargin, bool) {
	if explanation == nil || !finiteLiveReport(explanation.GetConservativeMargin()) || !finiteLiveReport(explanation.GetMarginHurdle()) || explanation.GetMarginHurdle() < 0 {
		return nil, false
	}
	values := map[string]bool{"DISPATCH_VALUE": true, "AVOIDED_PEAK_COST": true, "COMMITMENT_RELIABILITY_VALUE": true}
	costs := map[string]bool{"CHARGING_ENERGY": true, "INCREMENTAL_DEGRADATION": true, "PENALTY_EXPOSURE": true,
		"MEMBER_REWARD": true, "SUPPORT_AND_RISK_COST": true}
	margin := &reporting.ModeledMargin{ValueUSD: explanation.GetConservativeMargin(), HurdleUSD: explanation.GetMarginHurdle()}
	var upper float64
	var missingCosts []string
	for _, term := range explanation.GetTerms() {
		name := term.GetName()
		if !validMarginBounds(term) {
			return nil, false
		}
		if values[name] {
			if !sourcedMarginTerm(term) {
				return nil, false
			}
			upper += term.GetHigh()
			if name == "DISPATCH_VALUE" {
				margin.PriceProvenance = marginPriceProvenance(term.GetSource())
			}
			delete(values, name)
			continue
		}
		if !costs[name] {
			return nil, false
		}
		if term.GetUnavailable() {
			missingCosts = append(missingCosts, name)
		} else if !sourcedMarginTerm(term) {
			return nil, false
		}
		delete(costs, name)
	}
	if len(values) != 0 || len(costs) != 0 {
		return nil, false
	}
	if len(missingCosts) == 0 {
		return margin, true
	}
	if !validUpperMargin(upper, margin.PriceProvenance, missingCosts) {
		return nil, false
	}
	margin.ValueUSD = upper
	margin.Bound = "UPPER"
	margin.UnavailableCosts = missingCosts
	return margin, true
}

func validUpperMargin(upper float64, provenance string, missingCosts []string) bool {
	return len(missingCosts) == 5 && upper <= 0 && provenance != ""
}

func validMarginBounds(term *gridosv1.MarginTerm) bool {
	return finiteLiveReport(term.GetLow()) && finiteLiveReport(term.GetHigh()) && term.GetLow() <= term.GetHigh()
}

func sourcedMarginTerm(term *gridosv1.MarginTerm) bool {
	return !term.GetUnavailable() && term.GetSource() != "" && term.GetSource() != "UNAVAILABLE"
}

func marginPriceProvenance(source string) string {
	switch source {
	case "FROZEN_SIMULATED_PRICE":
		return "SIMULATED"
	case "FROZEN_PUBLIC_PRICE":
		return "CONFIRMED_PUBLIC"
	default:
		return ""
	}
}

func addLiveReportGap(report *reporting.StoredEvent, begin, end time.Time, reason string) {
	report.DataGaps = append(report.DataGaps, reporting.DataGap{Begin: begin, End: end, Reason: reason})
}

func fillLiveForecast(report *reporting.StoredEvent, frozen *gridosv1.OptimizationRequest, begin, end time.Time) {
	addLiveReportGap(report, begin, end, "delivery_method_unavailable")
	forecast := frozen.GetForecast()
	if forecast == nil {
		addLiveReportGap(report, begin, end, "frozen_forecast_unavailable")
		addLiveReportGap(report, begin, end, "baseline_unavailable")
		addLiveReportGap(report, begin, end, "baseline_confidence_unavailable")
		addLiveReportGap(report, begin, end, "availability_unavailable")
		return
	}
	baselineKWh, hours, baselineVersion, valid := frozenBaseline(frozen)
	if !valid {
		addLiveReportGap(report, begin, end, "baseline_unavailable")
		addLiveReportGap(report, begin, end, "baseline_confidence_unavailable")
		addLiveReportGap(report, begin, end, "availability_unavailable")
		return
	}
	report.Measurement = &reporting.Measurement{
		BaselineMWh:    baselineKWh / 1000,
		BaselineMW:     baselineKWh / 1000 / hours,
		BaselineMethod: baselineVersion,
		DeliveryMethod: "UNAVAILABLE",
	}
	report.Versions.Baseline = baselineVersion
	report.Versions.Forecast = baselineVersion
	report.Provenance = append(report.Provenance, "FROZEN_FORECAST")
	if coverage, available := frozenCoverage(forecast.GetSiteLoads()); available {
		report.Measurement.Confidence = coverage
	} else {
		addLiveReportGap(report, begin, end, "baseline_confidence_unavailable")
	}
	availability, version, valid := frozenAvailability(frozen)
	if !valid {
		addLiveReportGap(report, begin, end, "availability_unavailable")
		return
	}
	report.Measurement.Availability = availability
	report.Versions.Availability = version
}

func frozenCoverage(loads []*gridosv1.ForecastSiteLoad) (float64, bool) {
	if len(loads) == 0 {
		return 0, false
	}
	var coverage float64
	for index, load := range loads {
		value := load.GetLoadKwh()
		if value == nil || value.IntervalCoverage == nil || !finiteLiveReport(value.GetIntervalCoverage()) || value.GetIntervalCoverage() <= 0 || value.GetIntervalCoverage() > 1 || (index > 0 && value.GetIntervalCoverage() != coverage) {
			return 0, false
		}
		coverage = value.GetIntervalCoverage()
	}
	return coverage, true
}

type forecastCell struct {
	id    string
	begin time.Time
}

func frozenCells(ids []string, intervals []*gridosv1.OptimizationInterval) (map[forecastCell]bool, float64) {
	cells := make(map[forecastCell]bool, len(ids)*len(intervals))
	hours := 0.0
	for _, interval := range intervals {
		if interval.GetBeginTime() == nil || interval.GetEndTime() == nil {
			return nil, 0
		}
		begin := interval.GetBeginTime().AsTime()
		duration := interval.GetEndTime().AsTime().Sub(begin).Hours()
		if duration <= 0 {
			return nil, 0
		}
		hours += duration
		for _, id := range ids {
			if id == "" {
				return nil, 0
			}
			cells[forecastCell{id: id, begin: begin}] = true
		}
	}
	if len(cells) != len(ids)*len(intervals) {
		return nil, 0
	}
	return cells, hours
}

func frozenBaseline(frozen *gridosv1.OptimizationRequest) (float64, float64, string, bool) {
	ids := make([]string, 0, len(frozen.GetSites()))
	for _, site := range frozen.GetSites() {
		ids = append(ids, site.GetSiteId())
	}
	cells, hours := frozenCells(ids, frozen.GetIntervals())
	loads := frozen.GetForecast().GetSiteLoads()
	if len(cells) == 0 || len(loads) != len(cells) {
		return 0, 0, "", false
	}
	energy := 0.0
	version := ""
	for _, load := range loads {
		value := load.GetLoadKwh()
		if load.GetIntervalBeginTime() == nil {
			return 0, 0, "", false
		}
		cell := forecastCell{id: load.GetSiteId(), begin: load.GetIntervalBeginTime().AsTime()}
		if !cells[cell] || value == nil || value.GetModelVersion() == "" || !finiteLiveReport(value.GetValue()) || value.GetValue() < 0 || (version != "" && version != value.GetModelVersion()) {
			return 0, 0, "", false
		}
		delete(cells, cell)
		energy += value.GetValue()
		version = value.GetModelVersion()
	}
	return energy, hours, version, len(cells) == 0
}

func frozenAvailability(frozen *gridosv1.OptimizationRequest) (float64, string, bool) {
	ids := make([]string, 0, len(frozen.GetDevices()))
	for _, device := range frozen.GetDevices() {
		ids = append(ids, device.GetDeviceId())
	}
	cells, _ := frozenCells(ids, frozen.GetIntervals())
	availability := frozen.GetForecast().GetDeviceAvailability()
	if len(cells) == 0 || len(availability) != len(cells) {
		return 0, "", false
	}
	value := 0.0
	version := ""
	for _, device := range availability {
		probability := device.GetProbability()
		if device.GetIntervalBeginTime() == nil {
			return 0, "", false
		}
		cell := forecastCell{id: device.GetDeviceId(), begin: device.GetIntervalBeginTime().AsTime()}
		if !cells[cell] || probability == nil || probability.GetModelVersion() == "" || !finiteLiveReport(probability.GetValue()) || probability.GetValue() < 0 || probability.GetValue() > 1 || (version != "" && version != probability.GetModelVersion()) {
			return 0, "", false
		}
		delete(cells, cell)
		value += probability.GetValue() / float64(len(availability))
		version = probability.GetModelVersion()
	}
	return value, version, len(cells) == 0
}

func finiteLiveReport(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func reportExclusions(contents []byte) (map[string]uint64, error) {
	var values []struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(contents, &values); err != nil {
		return nil, err
	}
	result := make(map[string]uint64)
	for _, value := range values {
		result[value.Reason]++
	}
	return result, nil
}
