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
		stem := strings.TrimSuffix(entry.Name(), ".sql")
		if err = copyMigration(filepath.Join("database/migrations", entry.Name()), filepath.Join(temporary, stem+".up.sql")); err != nil {
			return "", err
		}
		if err = copyMigration(filepath.Join("database/rollback", entry.Name()), filepath.Join(temporary, stem+".down.sql")); err != nil {
			return "", err
		}
	}
	return temporary, nil
}

func copyMigration(source, destination string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, contents, 0o600)
}
