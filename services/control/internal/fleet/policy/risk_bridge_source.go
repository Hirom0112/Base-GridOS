package policy

import (
	"context"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	publiccontext "github.com/Hirom0112/Base-GridOS/services/control/internal/context"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RiskBridge struct {
	pool       *pgxpool.Pool
	sites      []*gridosv1.AuthorizedSite
	publicRoot string
	fleetFile  string
	store      *Store
}

type riskSource struct {
	members      map[string]string
	observations map[string]*gridosv1.TelemetryObservation
	gateways     map[string]RiskGateway
	public       publiccontext.Snapshot
	publicError  string
}

func NewRiskBridge(pool *pgxpool.Pool, sites []*gridosv1.AuthorizedSite, publicRoot, fleetFile string) *RiskBridge {
	return &RiskBridge{pool: pool, sites: sites, publicRoot: publicRoot, fleetFile: fleetFile, store: New(pool)}
}

func (bridge *RiskBridge) loadSources(ctx context.Context, at time.Time) (riskSource, error) {
	source := riskSource{members: make(map[string]string), observations: make(map[string]*gridosv1.TelemetryObservation),
		gateways: make(map[string]RiskGateway)}
	deviceIDs := make([]string, 0, len(bridge.sites))
	for _, site := range bridge.sites {
		for _, device := range site.GetDevices() {
			deviceIDs = append(deviceIDs, device.GetDeviceId())
		}
	}
	latest, err := storage.NewTelemetryStore(bridge.pool).Latest(ctx, deviceIDs, at.AddDate(0, 0, -7))
	if err != nil {
		return riskSource{}, err
	}
	for _, observation := range latest {
		source.observations[observation.GetDeviceId()] = observation
	}
	rows, err := bridge.pool.Query(ctx, `SELECT source.device_id,source.gateway_id,heartbeat.last_published_at
		FROM gateway_device_sources source JOIN gateway_heartbeats heartbeat USING (gateway_id)
		WHERE source.device_id = ANY($1::text[])`, deviceIDs)
	if err != nil {
		return riskSource{}, err
	}
	for rows.Next() {
		var deviceID string
		var gateway RiskGateway
		if err = rows.Scan(&deviceID, &gateway.EvidenceID, &gateway.LastPublishedAt); err != nil {
			rows.Close()
			return riskSource{}, err
		}
		source.gateways[deviceID] = gateway
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return riskSource{}, err
	}
	rows, err = bridge.pool.Query(ctx, `SELECT site_id,member_id FROM member_sites`)
	if err != nil {
		return riskSource{}, err
	}
	for rows.Next() {
		var siteID, memberID string
		if err = rows.Scan(&siteID, &memberID); err != nil {
			rows.Close()
			return riskSource{}, err
		}
		source.members[siteID] = memberID
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return riskSource{}, err
	}
	if bridge.publicRoot == "" {
		source.publicError = "public_context_unavailable"
		return source, nil
	}
	source.public, err = publiccontext.LoadPublic(bridge.publicRoot, at)
	if err != nil {
		source.publicError = "public_context_unavailable"
	}
	return source, nil
}
