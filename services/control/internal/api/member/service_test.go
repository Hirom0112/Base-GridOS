package member

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func memberDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("GRIDOS_DATABASE_URL")
	if url == "" {
		url = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_member_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migrations, err := filepath.Glob("../../../../../database/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(migrations)
	for _, path := range migrations {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, readErr = pool.Exec(ctx, string(contents)); readErr != nil {
			t.Fatalf("%s: %v", path, readErr)
		}
	}
	return pool
}

func TestMemberStatusScopeAndSiteState(t *testing.T) {
	ctx := context.Background()
	pool := memberDatabase(t)
	_, err := pool.Exec(ctx, `INSERT INTO member_sites(site_id, member_id, bound_at, source, provenance)
		VALUES ('site-1', 'member-1', now(), 'SIMULATED', '{"provenance":"SIMULATED"}'),
		('site-2', 'member-2', now(), 'SIMULATED', '{"provenance":"SIMULATED"}')`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-1", ObservedAt: now, OperatingState: fleet.OnGrid,
		Availability: fleet.Online, EnergyKWh: 8, BackupHoursCurrent: 4, BackupHours750W: 8})
	sites := []*gridosv1.AuthorizedSite{{Site: &gridosv1.Site{SiteId: "site-1"}, Devices: []*gridosv1.Device{{BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 10}}}}}
	service := NewService(pool, twin, sites, func() time.Time { return now })
	request := connect.NewRequest(&gridosv1.GetMemberStatusRequest{MemberId: "member-1", SiteId: "site-1"})
	request.Header().Set("X-GridOS-Role", "member")
	request.Header().Set("X-GridOS-Member-ID", "member-1")
	response, err := service.GetMemberStatus(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetStateOfEnergyPercent() != 80 || response.Msg.GetBackupHoursCurrent() != 4 || response.Msg.GetBackupHours_750W() != 8 {
		t.Fatalf("own site state = %+v", response.Msg)
	}
	request.Msg.SiteId = "site-2"
	if _, err := service.GetMemberStatus(ctx, request); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-member site code = %v", connect.CodeOf(err))
	}
	request.Msg.SiteId = "site-1"
	request.Header().Set("X-GridOS-Member-ID", "member-2")
	if _, err := service.GetMemberStatus(ctx, request); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-member principal code = %v", connect.CodeOf(err))
	}
	request.Header().Del("X-GridOS-Role")
	if _, err := service.GetMemberStatus(ctx, request); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("missing role code = %v", connect.CodeOf(err))
	}
}
