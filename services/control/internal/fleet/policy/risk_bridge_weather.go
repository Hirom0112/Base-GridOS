package policy

import (
	"path/filepath"
	"slices"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func weatherForSite(at time.Time, policy RiskPolicy, source riskSource, site *gridosv1.AuthorizedSite, fleetFile string) (*RiskWeather, string) {
	if source.publicError != "" {
		return nil, "weather_source_unmatched"
	}
	codes := policy.WeatherZoneUGC[filepath.Base(fleetFile)][site.GetSite().GetWeatherZone()]
	if len(codes.UGC) == 0 && len(codes.SAME) == 0 {
		return nil, "weather_source_unmatched"
	}
	active := false
	for _, alert := range source.public.Alerts {
		if at.Before(alert.Effective) || !at.Before(alert.Expires) {
			continue
		}
		active = true
		for _, code := range alert.UGC {
			if slices.Contains(codes.UGC, code) {
				return &RiskWeather{EvidenceID: alert.ID, AsOf: alert.Source.AsOf, Expires: alert.Expires, Active: true, Provenance: alert.Source.Provenance}, ""
			}
		}
		for _, code := range alert.SAME {
			if slices.Contains(codes.SAME, code) {
				return &RiskWeather{EvidenceID: alert.ID, AsOf: alert.Source.AsOf, Expires: alert.Expires, Active: true, Provenance: alert.Source.Provenance}, ""
			}
		}
	}
	if !active {
		return nil, "weather_no_active_alert"
	}
	return nil, "weather_source_unmatched"
}
