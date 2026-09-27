package report

type PartnerReport struct {
	EventID            string   `json:"event_id"`
	PlanVersion        uint64   `json:"plan_version"`
	RequestedMW        float64  `json:"requested_mw"`
	ApprovedMW         float64  `json:"approved_mw"`
	CommandedMW        float64  `json:"commanded_mw"`
	AcknowledgedMW     float64  `json:"acknowledged_mw"`
	DeliveredMW        *float64 `json:"delivered_mw,omitempty"`
	DeliveredMWh       *float64 `json:"delivered_mwh,omitempty"`
	ModeledNetValueUSD *float64 `json:"modeled_net_value_usd,omitempty"`
	ValueKind          string   `json:"value_kind,omitempty"`
}

func ForPartner(report EventReport) PartnerReport {
	partner := PartnerReport{
		EventID: report.EventID, PlanVersion: report.PlanVersion,
		RequestedMW: report.RequestedMW, ApprovedMW: report.ApprovedMW,
		CommandedMW: report.CommandedMW, AcknowledgedMW: report.AcknowledgedMW,
	}
	if report.Delivered != nil {
		deliveredMW, deliveredMWh := report.Delivered.DeliveredMW, report.Delivered.DeliveredMWh
		partner.DeliveredMW, partner.DeliveredMWh = &deliveredMW, &deliveredMWh
	}
	if report.Economics != nil {
		net := report.Economics.NetValueUSD
		partner.ModeledNetValueUSD = &net
		partner.ValueKind = "modeled_estimate"
	}
	return partner
}
