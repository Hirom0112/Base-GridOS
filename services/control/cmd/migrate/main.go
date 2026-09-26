package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate up|down")
		os.Exit(2)
	}
	databaseURL := os.Getenv("GRIDOS_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	migrations, err := migrationFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(migrations) }()
	sourceURL := (&url.URL{Scheme: "file", Path: migrations}).String()
	runner, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _, _ = runner.Close() }()
	switch os.Args[1] {
	case "up":
		err = runner.Up()
	case "down":
		err = runner.Down()
	default:
		fmt.Fprintln(os.Stderr, "usage: migrate up|down")
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func migrationFiles() (string, error) {
	temporary, err := os.MkdirTemp("", "gridos-migrations-")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir("database/migrations")
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join("database/migrations", entry.Name()))
		if readErr != nil {
			return "", readErr
		}
		name := strings.TrimSuffix(entry.Name(), ".sql") + ".up.sql"
		if writeErr := os.WriteFile(filepath.Join(temporary, name), contents, 0o600); writeErr != nil {
			return "", writeErr
		}
	}
	return temporary, nil
}
