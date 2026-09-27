package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	eventID := flag.String("event", "", "event identifier to replay")
	flag.Parse()
	if *eventID == "" || flag.NArg() != 0 {
		return errors.New("exactly one --event is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	databaseURL := environment("GRIDOS_DATABASE_URL", "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable")
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	client := gridosv1connect.NewOptimizationServiceClient(&http.Client{Transport: transport, Timeout: 20 * time.Second},
		environment("GRIDOS_DECISION_ADDR", "http://localhost:50061"), connect.WithGRPC())
	return run(ctx, *eventID, environment("GRIDOS_REPLAY_DIR", ".local/replay"), replay.PostgresSource{Pool: pool}, controlapi.NewConnectOptimizer(client), os.Stdout)
}

func run(ctx context.Context, eventID, directory string, source replay.Source, planner replay.Planner, output io.Writer) error {
	manifest, err := replay.Load(directory, eventID)
	if err != nil {
		return err
	}
	result, err := replay.Run(ctx, manifest, source, planner)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintln(output, result.Status); err != nil {
		return err
	}
	if result.Status == "IDENTICAL" {
		return nil
	}
	if err = json.NewEncoder(output).Encode(result.Differences); err != nil {
		return err
	}
	return errors.New("replay outcome differs")
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
