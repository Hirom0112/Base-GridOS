package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
)

func TestDownDropsEverythingUpCreated(t *testing.T) {
	t.Chdir(repositoryRoot(t))
	databaseURL := scratchDatabase(t)
	migrations, err := migrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(migrations) })
	runner, err := migrate.New((&url.URL{Scheme: "file", Path: migrations}).String(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runner.Close() })

	if err = runner.Up(); err != nil {
		t.Fatalf("first up: %v", err)
	}
	if tables := publicTables(t, databaseURL); tables == 0 {
		t.Fatal("up created no tables")
	}
	if !indexExists(t, databaseURL, "command_intents_event_issued_idx") {
		t.Fatal("up did not create command_intents_event_issued_idx")
	}
	if err = runner.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if tables := publicTables(t, databaseURL); tables != 0 {
		t.Fatalf("down left %d application tables behind", tables)
	}
	if err = runner.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if !indexExists(t, databaseURL, "command_acknowledgements_command_received_idx") {
		t.Fatal("second up did not recreate command_acknowledgements_command_received_idx")
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	if os.Getenv("TEST_SRCDIR") != "" {
		return filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"))
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Join(filepath.Dir(file), "../../../..")
}

func scratchDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_migrate_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	scratch, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	scratch.Path = "/" + name
	return scratch.String()
}

func publicTables(t *testing.T, databaseURL string) int {
	t.Helper()
	var count int
	err := connect(t, databaseURL).QueryRow(context.Background(), "SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func indexExists(t *testing.T, databaseURL, name string) bool {
	t.Helper()
	var exists bool
	err := connect(t, databaseURL).QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func connect(t *testing.T, databaseURL string) *pgx.Conn {
	t.Helper()
	ctx := context.Background()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close(ctx) })
	return connection
}
