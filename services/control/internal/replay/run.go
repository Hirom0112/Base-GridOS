package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Source interface {
	Load(context.Context, Manifest) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error)
}

type Planner interface {
	Optimize(context.Context, *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error)
}

type FieldDifference struct {
	Field    string          `json:"field"`
	Expected json.RawMessage `json:"expected"`
	Actual   json.RawMessage `json:"actual"`
}

type Result struct {
	Status      string            `json:"status"`
	Differences []FieldDifference `json:"differences"`
}

func Run(ctx context.Context, manifest Manifest, source Source, planner Planner) (Result, error) {
	if err := manifest.validate(); err != nil {
		return Result{}, err
	}
	if source == nil || planner == nil {
		return Result{}, errors.New("replay source and planner are required")
	}
	if err := manifest.verifyFiles(); err != nil {
		return Result{}, err
	}
	request, expected, err := source.Load(ctx, manifest)
	if err != nil {
		return Result{}, err
	}
	if request == nil || expected == nil || request.GetEventId() != manifest.EventID || expected.GetEventId() != manifest.EventID ||
		request.GetPlanVersion() != expected.GetPlanVersion() || request.GetReservePolicy().GetPolicyVersion() != manifest.PolicyVersion {
		return Result{}, errors.New("stored replay inputs do not match the manifest")
	}
	actual, err := planner.Optimize(ctx, request)
	if err != nil {
		return Result{}, err
	}
	if actual == nil {
		return Result{}, errors.New("replay planner returned no plan")
	}
	if proto.Equal(expected, actual) {
		return Result{Status: "IDENTICAL"}, nil
	}
	differences, err := diffPlans(expected, actual)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: "DIFFERENT", Differences: differences}, nil
}

func (manifest Manifest) verifyFiles() error {
	fleetHash, err := fileHash(manifest.FleetFile)
	if err != nil || fleetHash != manifest.FleetSHA256 {
		return fmt.Errorf("fleet hash mismatch: %w", errors.Join(err, errors.New("fleet file changed")))
	}
	if manifest.ScenarioFile == "" {
		if manifest.ScenarioSHA256 != "" {
			return errors.New("absent scenario has a hash")
		}
		return nil
	}
	scenarioHash, err := fileHash(manifest.ScenarioFile)
	if err != nil || scenarioHash != manifest.ScenarioSHA256 {
		return fmt.Errorf("scenario hash mismatch: %w", errors.Join(err, errors.New("scenario file changed")))
	}
	return nil
}

func diffPlans(expected, actual *gridosv1.DispatchPlan) ([]FieldDifference, error) {
	options := protojson.MarshalOptions{UseProtoNames: true}
	expectedJSON, err := options.Marshal(expected)
	if err != nil {
		return nil, err
	}
	actualJSON, err := options.Marshal(actual)
	if err != nil {
		return nil, err
	}
	var expectedFields, actualFields map[string]json.RawMessage
	if err = json.Unmarshal(expectedJSON, &expectedFields); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(actualJSON, &actualFields); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(expectedFields)+len(actualFields))
	for field := range expectedFields {
		names = append(names, field)
	}
	for field := range actualFields {
		if _, known := expectedFields[field]; !known {
			names = append(names, field)
		}
	}
	slices.Sort(names)
	differences := make([]FieldDifference, 0)
	for _, field := range names {
		if !bytes.Equal(expectedFields[field], actualFields[field]) {
			differences = append(differences, FieldDifference{Field: field, Expected: expectedFields[field], Actual: actualFields[field]})
		}
	}
	return differences, nil
}
