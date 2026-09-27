package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const localAuthStatus = "PENDING-LIVE"
const localStepUpStatus = "PENDING-LIVE"

var localRoles = []string{"operator", "approver", "analyst", "partner", "service", "member"}

type server struct {
	fixtureRoot string
}

type identity struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "record" {
		return runRecord(os.Args[2:])
	}
	authMode := os.Getenv("GRIDOS_AUTH_MODE")
	if authMode != "" && authMode != "local" {
		return fmt.Errorf("GRIDOS_AUTH_MODE must be local, got %q", authMode)
	}
	fixtureRoot := os.Getenv("GRIDOS_FIXTURE_ROOT")
	if fixtureRoot == "" {
		fixtureRoot = filepath.Join("testdata", "fixtures", "api")
	}
	address := os.Getenv("GRIDOS_MOCKAPI_ADDRESS")
	if address == "" {
		address = ":8080"
	}
	httpServer := http.Server{
		Addr:              address,
		Handler:           server{fixtureRoot: fixtureRoot},
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return httpServer.ListenAndServe()
}

func (s server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	setCORS(response)
	if request.Method == http.MethodOptions {
		response.WriteHeader(http.StatusNoContent)
		return
	}
	if request.URL.Path == "/_gridos/local-identities" {
		s.serveIdentities(response, request)
		return
	}
	if request.URL.Path == "/local/step-up" {
		s.serveStepUp(response, request)
		return
	}
	s.serveFixture(response, request)
}

func setCORS(response http.ResponseWriter) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	response.Header().Set("Access-Control-Allow-Headers", "content-type,x-gridos-role,x-gridos-permissions")
	response.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
}

func (s server) serveIdentities(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	identities := make([]identity, 0, len(localRoles))
	for _, role := range localRoles {
		identities = append(identities, identity{Role: role, Permissions: []string{"site_location"}, Status: localAuthStatus})
	}
	writeJSON(response, identities)
}

func (s server) serveStepUp(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-GridOS-Auth-Status", localStepUpStatus)
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	role := request.Header.Get("X-GridOS-Role")
	if role != "approver" && role != "operator" {
		http.Error(response, "approver or operator role required", http.StatusForbidden)
		return
	}
	var input struct {
		Action      string `json:"action"`
		EventID     string `json:"event_id"`
		PlanVersion uint64 `json:"plan_version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || input.EventID == "" {
		http.Error(response, "invalid step-up request", http.StatusBadRequest)
		return
	}
	if input.Action == "APPROVE_EVENT" && role != "approver" {
		http.Error(response, "approver role required", http.StatusForbidden)
		return
	}
	if input.Action != "APPROVE_EVENT" && input.Action != "EMERGENCY_STOP" || input.Action == "APPROVE_EVENT" && input.PlanVersion == 0 || input.Action == "EMERGENCY_STOP" && input.PlanVersion != 0 {
		http.Error(response, "invalid step-up action", http.StatusBadRequest)
		return
	}
	key := os.Getenv("GRIDOS_STEP_UP_KEY")
	if len(key) < 32 {
		http.Error(response, "step-up signer unavailable", http.StatusServiceUnavailable)
		return
	}
	assertion, err := signStepUp(key, "local-"+role, input.Action, input.EventID, input.PlanVersion)
	if err != nil {
		http.Error(response, "step-up signer unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(struct {
		Assertion string `json:"assertion"`
	}{assertion}); err != nil {
		return
	}
}

func (s server) serveFixture(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	role := request.Header.Get("X-GridOS-Role")
	if role == "" {
		role = "operator"
	}
	if !slices.Contains(localRoles, role) {
		http.Error(response, "unknown local role", http.StatusForbidden)
		return
	}
	service, method, err := procedureParts(request.URL.Path)
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if service == "DispatchService" && method == "ApproveEvent" && role != "approver" {
		http.Error(response, "approver role required", http.StatusForbidden)
		return
	}
	if err := validateRequest(response, request); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	content, err := os.ReadFile(filepath.Join(s.fixtureRoot, service, method+".json"))
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(response, request)
		return
	}
	if err != nil {
		http.Error(response, "fixture unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("X-GridOS-Role", role)
	response.WriteHeader(http.StatusOK)
	if _, err := response.Write(content); err != nil {
		return
	}
}

func procedureParts(path string) (string, string, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "gridos.v1.") {
		return "", "", errors.New("invalid procedure")
	}
	service := strings.TrimPrefix(parts[0], "gridos.v1.")
	if !safeName(service) || !safeName(parts[1]) {
		return "", "", errors.New("invalid procedure")
	}
	return service, parts[1], nil
}

func safeName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' && character < 'a' || character > 'z' {
			return false
		}
	}
	return true
}

func validateRequest(response http.ResponseWriter, request *http.Request) error {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 1<<20))
	var value map[string]json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		_ = request.Body.Close()
		return errors.New("invalid JSON request")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		_ = request.Body.Close()
		return errors.New("request must contain one JSON value")
	}
	return request.Body.Close()
}

func writeJSON(response http.ResponseWriter, value []identity) {
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(value); err != nil {
		http.Error(response, "response encoding failed", http.StatusInternalServerError)
	}
}
