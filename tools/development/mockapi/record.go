package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type recordingIndex struct {
	Screens  map[string][]string         `json:"screens"`
	Requests map[string]recordingRequest `json:"requests"`
}

type recordingRequest struct {
	Role string          `json:"role"`
	Body json.RawMessage `json:"body"`
}

type recordedFixture struct {
	path    string
	content []byte
}

func runRecord(arguments []string) error {
	flags := flag.NewFlagSet("record", flag.ContinueOnError)
	baseURL := flags.String("base-url", "", "")
	fixtureRoot := flags.String("fixture-root", filepath.Join("testdata", "fixtures", "api"), "")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *baseURL == "" {
		return errors.New("record requires --base-url")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	return recordFixtures(*fixtureRoot, *baseURL, client)
}

func recordFixtures(fixtureRoot, baseURL string, client *http.Client) error {
	index, err := loadRecordingIndex(filepath.Join(fixtureRoot, "INDEX.json"))
	if err != nil {
		return err
	}
	methodNames, err := indexedMethods(index)
	if err != nil {
		return err
	}
	recorded := make([]recordedFixture, 0, len(methodNames))
	for _, methodName := range methodNames {
		fixture, err := callMethod(baseURL, fixtureRoot, methodName, index.Requests[methodName], client)
		if err != nil {
			return err
		}
		recorded = append(recorded, fixture)
	}
	for _, fixture := range recorded {
		if err := writeFixture(fixture); err != nil {
			return err
		}
	}
	return nil
}

func loadRecordingIndex(path string) (recordingIndex, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return recordingIndex{}, err
	}
	var index recordingIndex
	if err := json.Unmarshal(content, &index); err != nil {
		return recordingIndex{}, err
	}
	return index, nil
}

func indexedMethods(index recordingIndex) ([]string, error) {
	unique := make(map[string]bool)
	for _, methods := range index.Screens {
		for _, methodName := range methods {
			request, found := index.Requests[methodName]
			if !found || len(request.Body) == 0 {
				return nil, fmt.Errorf("INDEX request missing for %s", methodName)
			}
			if request.Role == "" {
				return nil, fmt.Errorf("INDEX role missing for %s", methodName)
			}
			unique[methodName] = true
		}
	}
	methods := make([]string, 0, len(unique))
	for methodName := range unique {
		methods = append(methods, methodName)
	}
	sort.Slice(methods, func(left, right int) bool {
		leftRank := methodRank(methods[left])
		rightRank := methodRank(methods[right])
		if leftRank == rightRank {
			return methods[left] < methods[right]
		}
		return leftRank < rightRank
	})
	return methods, nil
}

func methodRank(methodName string) int {
	lifecycle := map[string]int{
		"gridos.v1.DispatchService.CreateEventRequest": 0,
		"gridos.v1.DispatchService.GetEvent":           1,
		"gridos.v1.DispatchService.ApproveEvent":       2,
		"gridos.v1.DispatchService.LaunchEvent":        3,
	}
	rank, found := lifecycle[methodName]
	if found {
		return rank
	}
	return -1
}

func callMethod(baseURL, fixtureRoot, methodName string, indexedRequest recordingRequest, client *http.Client) (recordedFixture, error) {
	serviceName, procedure, err := methodProcedure(methodName)
	if err != nil {
		return recordedFixture{}, err
	}
	endpoint, err := url.JoinPath(baseURL, procedure)
	if err != nil {
		return recordedFixture{}, err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(indexedRequest.Body))
	if err != nil {
		return recordedFixture{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GridOS-Role", indexedRequest.Role)
	response, err := client.Do(request)
	if err != nil {
		return recordedFixture{}, err
	}
	content, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		return recordedFixture{}, readErr
	}
	if closeErr != nil {
		return recordedFixture{}, closeErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return recordedFixture{}, fmt.Errorf("%s returned %s: %s", methodName, response.Status, content)
	}
	if len(content) > 1<<20 {
		return recordedFixture{}, fmt.Errorf("%s response exceeds 1 MiB", methodName)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, content); err != nil {
		return recordedFixture{}, fmt.Errorf("%s returned invalid JSON: %w", methodName, err)
	}
	compact.WriteByte('\n')
	path := filepath.Join(fixtureRoot, serviceName, strings.TrimPrefix(procedure, "/gridos.v1."+serviceName+"/")+".json")
	return recordedFixture{path: path, content: compact.Bytes()}, nil
}

func methodProcedure(methodName string) (string, string, error) {
	separator := strings.LastIndexByte(methodName, '.')
	if separator < 0 {
		return "", "", fmt.Errorf("invalid method %q", methodName)
	}
	service := methodName[:separator]
	method := methodName[separator+1:]
	serviceName := strings.TrimPrefix(service, "gridos.v1.")
	if serviceName == service || !safeName(serviceName) || !safeName(method) {
		return "", "", fmt.Errorf("invalid method %q", methodName)
	}
	return serviceName, "/" + service + "/" + method, nil
}

func writeFixture(fixture recordedFixture) error {
	if err := os.MkdirAll(filepath.Dir(fixture.path), 0o755); err != nil {
		return err
	}
	temporary := fixture.path + ".tmp"
	if err := os.WriteFile(temporary, fixture.content, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, fixture.path)
}
