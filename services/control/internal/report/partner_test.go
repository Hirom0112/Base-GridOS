package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPartnerViewContainsAggregatesWithoutHouseholdDetails(t *testing.T) {
	report := EventReport{
		EventID: "event-1", PlanVersion: 2, RequestedMW: 10, ApprovedMW: 9, CommandedMW: 8, AcknowledgedMW: 7,
		Delivered:        &Delivered{DeliveredMW: 6, DeliveredMWh: 12, UncertainIntervals: []UncertainInterval{{DeviceID: "private-device"}}},
		ExcludedByReason: map[string]uint64{"RESERVE": 3}, Economics: &ModeledEconomics{NetValueUSD: 45, ValueKind: "modeled_estimate"},
		Assumptions: []string{"private site_id site-1 travel away"},
	}
	content, err := json.Marshal(ForPartner(report))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"site_id", "device_id", "private-device", "site-1", "travel", "away"} {
		if strings.Contains(string(content), private) {
			t.Fatalf("partner report exposed %q: %s", private, content)
		}
	}
	var public struct {
		EventID            string  `json:"event_id"`
		DeliveredMW        float64 `json:"delivered_mw"`
		ModeledNetValueUSD float64 `json:"modeled_net_value_usd"`
	}
	if err = json.Unmarshal(content, &public); err != nil {
		t.Fatal(err)
	}
	if public.EventID != "event-1" || public.DeliveredMW != 6 || public.ModeledNetValueUSD != 45 {
		t.Fatalf("partner aggregate missing: %+v", public)
	}
}

func TestPartnerCoverageDistinguishesMissingFromMeasuredZero(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		coverage   float64
		deliveryOK bool
	}{
		{name: "unknown", coverage: 0},
		{name: "measured zero", coverage: 0.5, deliveryOK: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			content, err := json.Marshal(ForPartner(EventReport{EventID: "event-coverage", Delivered: &Delivered{Completeness: testCase.coverage}}))
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(content, &fields); err != nil {
				t.Fatal(err)
			}
			var coverage float64
			if err := json.Unmarshal(fields["delivery_coverage"], &coverage); err != nil || coverage != testCase.coverage {
				t.Fatalf("partner coverage = %s", content)
			}
			_, power := fields["delivered_mw"]
			_, energy := fields["delivered_mwh"]
			if power != testCase.deliveryOK || energy != testCase.deliveryOK {
				t.Fatalf("partner delivery visibility = %s", content)
			}
		})
	}
}
