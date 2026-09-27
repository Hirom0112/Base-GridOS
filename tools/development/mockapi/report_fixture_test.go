package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestFixturesCaptureMeasuredReportShortfalls(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "fixtures", "api")
	response := new(gridosv1.GetEventReportResponse)
	readPlanningFixture(t, root, "ReportService/GetEventReport.live.json", response)
	type dataGap struct {
		Reason string `json:"Reason"`
	}
	var report struct {
		EventID            string            `json:"EventID"`
		MemberRewardsCents int64             `json:"MemberRewardsCents"`
		Margin             json.RawMessage   `json:"Margin"`
		Provenance         []string          `json:"Provenance"`
		DataGaps           []dataGap         `json:"DataGaps"`
		PlannedShortfall   []json.RawMessage `json:"planned_shortfall"`
		DeliveryShortfall  []struct {
			Coverage             float64  `json:"coverage"`
			MeasuredDeliveredKWh *float64 `json:"measured_delivered_kwh"`
			ShortfallKWh         *float64 `json:"shortfall_kwh"`
			ValueKind            string   `json:"value_kind"`
		} `json:"delivery_shortfall"`
	}
	if err := json.Unmarshal([]byte(response.GetReportJson()), &report); err != nil {
		t.Fatal(err)
	}
	if report.EventID == "" || len(report.PlannedShortfall) == 0 || len(report.DeliveryShortfall) == 0 {
		t.Fatal("recorded report lacks planned or delivery shortfall")
	}
	if string(report.Margin) == "null" && !slices.Contains(report.DataGaps, dataGap{Reason: "margin_unavailable"}) {
		t.Fatal("recorded report omits margin without a margin_unavailable gap")
	}
	if report.MemberRewardsCents == 0 && !slices.Contains(report.Provenance, "REWARD_LEDGER") {
		t.Fatal("recorded report shows zero rewards without reward ledger provenance")
	}
	for _, interval := range report.DeliveryShortfall {
		if interval.ValueKind == "MEASURED" && interval.Coverage > 0 && interval.MeasuredDeliveredKWh != nil && interval.ShortfallKWh != nil {
			return
		}
	}
	t.Fatal("recorded report lacks measured delivery shortfall")
}
