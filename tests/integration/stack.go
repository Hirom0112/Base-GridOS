package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	gatewayToken     = "Bearer integration-gateway"
	gatewayID        = "integration-gateway"
	temporalAddress  = "127.0.0.1:7233"
	composeTimeout   = 2 * time.Minute
	listenTimeout    = 30 * time.Second
	stopTimeout      = 5 * time.Second
	defaultAdminURL  = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	logTailBytes     = 4000
	listenProbeDelay = 25 * time.Millisecond
	clientTimeout    = 30 * time.Second
)

var (
	buildOnce sync.Once
	buildDir  string
	buildErr  error
)

type binaries struct {
	gateway, control, worker, migrate string
}

type stack struct {
	scenario     Scenario
	root         string
	databaseName string
	databaseURL  string
	pool         *pgxpool.Pool
	controlURL   string
	gatewayURL   string
	decisionURL  string
	gatewayDB    string
	logDir       string
	processes    map[string]*process
	client       *http.Client
}

type process struct {
	name    string
	env     []string
	program string
	args    []string
	log     string
	command *exec.Cmd
	exited  chan struct{}
}

func startStack(t *testing.T, scenarioName string) *stack {
	t.Helper()
	root := repositoryRoot(t)
	requireInfrastructure(t, root)
	built := buildBinaries(t, root)
	scenario := loadScenario(t, root, scenarioName)
	ctx := context.Background()
	name := fmt.Sprintf("gridos_integration_%d", time.Now().UnixNano())
	stack := &stack{
		scenario: scenario, root: root, databaseName: name, databaseURL: databaseURL(t, name),
		logDir: t.TempDir(), processes: make(map[string]*process), client: &http.Client{Timeout: clientTimeout},
	}
	stack.gatewayDB = filepath.Join(stack.logDir, "gateway.db")
	createDatabase(t, ctx, name)
	t.Cleanup(func() { dropDatabase(t, context.Background(), name) })
	stack.migrate(t, built.migrate)
	pool, err := pgxpool.New(ctx, stack.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	stack.pool = pool
	decisionAddress, gatewayAddress, controlAddress := freeAddress(t), freeAddress(t), freeAddress(t)
	stack.decisionURL, stack.gatewayURL, stack.controlURL = "http://"+decisionAddress, "http://"+gatewayAddress, "http://"+controlAddress
	stack.start(t, "decision", nil, "uv", "run", "--project", "services/decision", "python", "-m", "gridos.server", "--port", port(t, decisionAddress))
	stack.start(t, "gateway", []string{"GRIDOS_GATEWAY_TOKEN=" + gatewayToken}, built.gateway,
		"--scenario", filepath.Join("testdata/scenarios", scenarioName+".yaml"), "--address", gatewayAddress, "--database", stack.gatewayDB, "--gateway-id", gatewayID)
	controlEnv := []string{
		"GRIDOS_CONTROL_ADDRESS=" + controlAddress, "GRIDOS_DATABASE_URL=" + stack.databaseURL, "GRIDOS_GATEWAY_ADDR=" + stack.gatewayURL,
		"GRIDOS_DECISION_ADDR=" + stack.decisionURL, "GRIDOS_FLEET=" + scenario.Fleet.Path, "GRIDOS_GATEWAY_TOKEN=" + gatewayToken, "TEMPORAL_ADDRESS=" + temporalAddress,
	}
	stack.start(t, "control", controlEnv, built.control)
	stack.start(t, "worker", controlEnv, built.worker)
	stack.waitListening(t, "decision", decisionAddress)
	stack.waitListening(t, "gateway", gatewayAddress)
	stack.waitListening(t, "control", controlAddress)
	stack.assertRunning(t, "worker")
	return stack
}

func requireInfrastructure(t *testing.T, root string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), composeTimeout)
	defer cancel()
	compose := exec.CommandContext(ctx, "docker", "compose", "-f", "infrastructure/local/compose.yaml", "up", "-d", "--wait")
	compose.Dir = root
	if output, err := compose.CombinedOutput(); err != nil {
		t.Skipf("Docker compose is unavailable, skipping the integration stack: %v: %s", err, tail(output))
	}
	admin, err := pgx.Connect(ctx, adminDatabaseURL())
	if err != nil {
		t.Skipf("PostgreSQL is unreachable at %s, skipping the integration stack: %v", adminDatabaseURL(), err)
	}
	_ = admin.Close(ctx)
	connection, err := net.DialTimeout("tcp", temporalAddress, time.Second)
	if err != nil {
		t.Skipf("Temporal is unreachable at %s, skipping the integration stack: %v", temporalAddress, err)
	}
	_ = connection.Close()
}

func buildBinaries(t *testing.T, root string) binaries {
	t.Helper()
	buildOnce.Do(func() {
		buildDir, buildErr = os.MkdirTemp("", "gridos-integration-")
		if buildErr != nil {
			return
		}
		for _, name := range []string{"gateway-simulator", "control", "worker", "migrate"} {
			pkg := "./services/control/cmd/" + name
			if name == "gateway-simulator" {
				pkg = "./services/gateway-simulator/cmd/gateway-simulator"
			}
			build := exec.Command("go", "build", "-o", filepath.Join(buildDir, name), pkg)
			build.Dir = root
			if output, err := build.CombinedOutput(); err != nil {
				buildErr = fmt.Errorf("build %s: %w: %s", name, err, output)
				return
			}
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return binaries{
		gateway: filepath.Join(buildDir, "gateway-simulator"), control: filepath.Join(buildDir, "control"),
		worker: filepath.Join(buildDir, "worker"), migrate: filepath.Join(buildDir, "migrate"),
	}
}

func removeBinaries() {
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
}

func adminDatabaseURL() string {
	if value := os.Getenv("GRIDOS_DATABASE_URL"); value != "" {
		return value
	}
	return defaultAdminURL
}

func databaseURL(t *testing.T, name string) string {
	t.Helper()
	parsed, err := url.Parse(adminDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + name
	return parsed.String()
}

func createDatabase(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	admin, err := pgx.Connect(ctx, adminDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close(ctx) }()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
}

func dropDatabase(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	admin, err := pgx.Connect(ctx, adminDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close(ctx) }()
	if _, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
}

func assertDatabaseDropped(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	admin, err := pgx.Connect(ctx, adminDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close(ctx) }()
	var exists bool
	if err = admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("database %s survived teardown", name)
	}
}

func (stack *stack) migrate(t *testing.T, program string) {
	t.Helper()
	migrate := exec.Command(program, "up")
	migrate.Dir = stack.root
	migrate.Env = append(os.Environ(), "GRIDOS_DATABASE_URL="+stack.databaseURL)
	if output, err := migrate.CombinedOutput(); err != nil {
		t.Fatalf("migrate: %v: %s", err, output)
	}
}

func (stack *stack) start(t *testing.T, name string, env []string, program string, args ...string) {
	t.Helper()
	process := &process{name: name, env: env, program: program, args: args, log: filepath.Join(stack.logDir, name+".log")}
	stack.processes[name] = process
	stack.launch(t, process)
	t.Cleanup(process.stop)
}

func (stack *stack) launch(t *testing.T, process *process) {
	t.Helper()
	logFile, err := os.OpenFile(process.log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logFile.Close() }()
	command := exec.Command(process.program, process.args...)
	command.Dir = stack.root
	command.Env = append(os.Environ(), process.env...)
	command.Stdout, command.Stderr = logFile, logFile
	if err = command.Start(); err != nil {
		t.Fatalf("start %s: %v", process.name, err)
	}
	exited := make(chan struct{})
	process.command, process.exited = command, exited
	go func() {
		_ = command.Wait()
		close(exited)
	}()
}

func (process *process) stop() {
	if process.command == nil {
		return
	}
	select {
	case <-process.exited:
		return
	default:
	}
	_ = process.command.Process.Signal(syscall.SIGTERM)
	select {
	case <-process.exited:
	case <-time.After(stopTimeout):
		_ = process.command.Process.Kill()
		<-process.exited
	}
}

func (stack *stack) waitListening(t *testing.T, name, address string) {
	t.Helper()
	process := stack.processes[name]
	deadline := time.Now().Add(listenTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-process.exited:
			t.Fatalf("%s exited before listening on %s:\n%s", name, address, stack.logTail(t, name))
		default:
		}
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(listenProbeDelay)
	}
	t.Fatalf("%s did not listen on %s within %s:\n%s", name, address, listenTimeout, stack.logTail(t, name))
}

func (stack *stack) assertRunning(t *testing.T, name string) {
	t.Helper()
	select {
	case <-stack.processes[name].exited:
		t.Fatalf("%s exited:\n%s", name, stack.logTail(t, name))
	default:
	}
}

func (stack *stack) logTail(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(stack.processes[name].log)
	if err != nil {
		return err.Error()
	}
	return tail(contents)
}

func tail(contents []byte) string {
	if len(contents) > logTailBytes {
		contents = contents[len(contents)-logTailBytes:]
	}
	return string(contents)
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func port(t *testing.T, address string) string {
	t.Helper()
	_, value, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
