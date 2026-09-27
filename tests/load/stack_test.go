package load

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const loadGatewayToken = "Bearer load-gateway"

type loadStack struct {
	root        string
	name        string
	databaseURL string
	database    *pgxpool.Pool
	controlURL  string
	logDir      string
}

func startLoadStack(t *testing.T) *loadStack {
	t.Helper()
	root := loadRoot(t)
	name := fmt.Sprintf("gridos_load_%d", time.Now().UnixNano())
	adminURL := os.Getenv("GRIDOS_LOAD_ADMIN_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("isolated PostgreSQL prerequisite: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if _, dropErr := admin.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); dropErr != nil {
			t.Error(dropErr)
		}
	})
	parsed.Path = "/" + name
	databaseURL := parsed.String()
	binDir := t.TempDir()
	migrate := buildLoadBinary(t, root, binDir, "migrate", "./services/control/cmd/migrate")
	control := buildLoadBinary(t, root, binDir, "control", "./services/control/cmd/control")
	command := exec.CommandContext(ctx, migrate, "up")
	command.Dir = root
	command.Env = append(os.Environ(), "GRIDOS_DATABASE_URL="+databaseURL)
	if output, runErr := command.CombinedOutput(); runErr != nil {
		t.Fatalf("migrate isolated database: %v: %s", runErr, output)
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	address := loadAddress(t)
	stack := &loadStack{root: root, name: name, databaseURL: databaseURL, database: pool, controlURL: "http://" + address, logDir: t.TempDir()}
	stack.start(t, "control", address, control, []string{
		"GRIDOS_CONTROL_ADDRESS=" + address,
		"GRIDOS_DATABASE_URL=" + databaseURL,
		"GRIDOS_FLEET=" + filepath.Join(root, "testdata/fleets/austin-5000.jsonl"),
		"GRIDOS_GATEWAY_TOKEN=" + loadGatewayToken,
		"GRIDOS_TASK_QUEUE=" + name,
		"GRIDOS_DECISION_ADDR=http://127.0.0.1:1",
	})
	return stack
}

func buildLoadBinary(t *testing.T, root, binDir, name, path string) string {
	t.Helper()
	bin := filepath.Join(binDir, name)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-o", bin, path)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", name, err, output)
	}
	return bin
}

func (stack *loadStack) start(t *testing.T, name, address, binary string, env []string, args ...string) {
	t.Helper()
	logPath := filepath.Join(stack.logDir, name+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, args...)
	command.Dir = stack.root
	command.Env = append(os.Environ(), env...)
	command.Stdout, command.Stderr = logFile, logFile
	if err = command.Start(); err != nil {
		_ = logFile.Close()
		t.Fatal(err)
	}
	_ = logFile.Close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
	})
	if address == "" {
		select {
		case exitErr := <-done:
			contents, _ := os.ReadFile(logPath)
			t.Fatalf("%s exited: %v: %s", name, exitErr, contents)
		case <-time.After(500 * time.Millisecond):
		}
		return
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case exitErr := <-done:
			contents, _ := os.ReadFile(logPath)
			t.Fatalf("%s exited: %v: %s", name, exitErr, contents)
		default:
		}
		connection, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	contents, _ := os.ReadFile(logPath)
	t.Fatalf("%s did not listen: %s", name, contents)
}

func loadRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func loadAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}
