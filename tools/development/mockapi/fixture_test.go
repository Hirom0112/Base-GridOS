package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type fixtureIndex struct {
	Screens  map[string][]string                  `json:"screens"`
	Variants map[string]map[string]fixtureVariant `json:"variants"`
}

type fixtureVariant struct {
	Role        string          `json:"role"`
	Permissions string          `json:"permissions"`
	Body        json.RawMessage `json:"body"`
}

func TestFixturesCapturePlanningCases(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "fixtures", "api")
	explanation := new(gridosv1.GetPlanExplanationResponse)
	readPlanningFixture(t, root, "DispatchService/GetPlanExplanation.json", explanation)
	if explanation.GetObjectiveBreakdown() == nil || explanation.GetReserveHeldBackKwh() <= 0 || len(explanation.GetConstraintMargins()) == 0 {
		t.Fatal("recorded explanation lacks objective, reserve, or margins")
	}
	forecast := new(gridosv1.ForecastResponse)
	readPlanningFixture(t, root, "OptimizationService/Forecast.json", forecast)
	if len(forecast.GetSiteLoads()) == 0 || len(forecast.GetDeviceAvailability()) == 0 || forecast.GetSiteLoads()[0].GetIntervalBeginTime() == nil ||
		forecast.GetSiteLoads()[0].GetLoadKwh().GetLower() >= forecast.GetSiteLoads()[0].GetLoadKwh().GetUpper() {
		t.Fatal("recorded forecast lacks intervals or bounds")
	}
	fallback := new(gridosv1.OptimizeResponse)
	readPlanningFixture(t, root, "OptimizationService/Optimize.json", fallback)
	if !fallback.GetPlan().GetFallbackUsed() || fallback.GetPlan().GetFallbackReason() == "" || len(fallback.GetPlan().GetShortfalls()) == 0 {
		t.Fatal("recorded plan does not show a quantified fallback")
	}
	unsafe := new(gridosv1.ValidateUnsafeAlternativeResponse)
	readPlanningFixture(t, root, "DispatchService/ValidateUnsafeAlternative.json", unsafe)
	if unsafe.GetApproved() || len(unsafe.GetViolations()) == 0 || unsafe.GetOperatorExplanation() == "" {
		t.Fatal("recorded unsafe alternative lacks a violation explanation")
	}
}

func TestFixturesCaptureContextGeo(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "fixtures", "api")
	market := new(gridosv1.GetMarketContextResponse)
	readPlanningFixture(t, root, "ContextService/GetMarketContext.json", market)
	if len(market.GetDayAheadPrices()) == 0 || len(market.GetRealTimePrices()) == 0 || len(market.GetSystemLoads()) == 0 || market.GetDayAheadPrices()[0].GetSource().GetAsOf() == nil {
		t.Fatal("recorded market context lacks priced and sourced public data")
	}
	weather := new(gridosv1.GetWeatherContextResponse)
	readPlanningFixture(t, root, "ContextService/GetWeatherContext.json", weather)
	if len(weather.GetForecasts()) == 0 || weather.GetForecasts()[0].GetSource().GetAsOf() == nil {
		t.Fatal("recorded weather context lacks sourced forecasts")
	}
	outage := new(gridosv1.GetOutageRiskResponse)
	readPlanningFixture(t, root, "ContextService/GetOutageRisk.json", outage)
	if len(outage.GetRates()) == 0 || outage.GetRates()[0].GetSource().GetAsOf() == nil {
		t.Fatal("recorded outage risk lacks sourced rates")
	}
	windows := new(gridosv1.ListDispatchWindowsResponse)
	readPlanningFixture(t, root, "ContextService/ListDispatchWindows.json", windows)
	if len(windows.GetWindows()) < 2 || windows.GetWindows()[0].GetValueKind() != "modeled_estimate" {
		t.Fatal("recorded dispatch windows lack modeled ranking")
	}
	for _, resolution := range []uint64{5, 6, 7} {
		cells := new(gridosv1.ListCellsResponse)
		readPlanningFixture(t, root, "GeoService/ListCells.res"+strconv.FormatUint(resolution, 10)+".json", cells)
		foundResolution := false
		for _, cell := range cells.GetCells() {
			index, err := strconv.ParseUint(cell.GetH3Cell(), 16, 64)
			if err != nil || cell.GetSiteCount() < 5 || cell.GetProvenance() != gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED || (index>>52)&15 > resolution {
				t.Fatalf("recorded resolution %d cell violates privacy or provenance: %s", resolution, cell.GetH3Cell())
			}
			foundResolution = foundResolution || (index>>52)&15 == resolution
		}
		if !foundResolution {
			t.Fatalf("recorded resolution %d has no cells at the requested resolution", resolution)
		}
	}
	path := []string{"market:ERCOT", "load_zone:LZ_AEN", "utility:LZ_AEN", "substation:LZ_AEN:85489e37fffffff", "feeder:LZ_AEN:86489e367ffffff"}
	for index, level := range []string{"root", "market", "load_zone", "utility", "substation", "feeder"} {
		response := new(gridosv1.DrilldownResponse)
		name := "GeoService/Drilldown." + level + ".json"
		if level == "root" {
			name = "GeoService/Drilldown.json"
		}
		readPlanningFixture(t, root, name, response)
		if level == "feeder" {
			if len(response.GetSites()) == 0 {
				t.Fatal("recorded drilldown path has no authorized sites")
			}
			continue
		}
		parent := ""
		if index > 0 {
			parent = path[index-1]
		}
		found := false
		for _, node := range response.GetNodes() {
			found = found || node.GetId() == path[index] && node.GetParentId() == parent && node.GetProvenance() == gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED
		}
		if !found {
			t.Fatalf("recorded drilldown path breaks at %s", level)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index fixtureIndex
	if err := json.Unmarshal(content, &index); err != nil {
		t.Fatal(err)
	}
	if index.Variants["gridos.v1.GeoService.Drilldown"]["feeder"].Permissions != "site_location" {
		t.Fatal("recorded exact site drilldown lacks site_location permission")
	}
}

func TestFixturesCapturePublishedReportViews(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "fixtures", "api")
	type reportValue struct {
		EventID            string `json:"EventID"`
		MemberRewardsCents int64  `json:"MemberRewardsCents"`
		Margin             struct {
			ValueUSD  float64 `json:"ValueUSD"`
			ValueKind string  `json:"ValueKind"`
		} `json:"Margin"`
	}
	values := []struct {
		name   string
		id     string
		reward int64
		margin float64
	}{
		{"ReportService/GetEventReport.json", "event-report-a", 725, -3.25},
		{"ReportService/GetEventReport.comparison_peer.json", "event-report-b", 1250, 1.5},
	}
	for _, expected := range values {
		response := new(gridosv1.GetEventReportResponse)
		readPlanningFixture(t, root, expected.name, response)
		var value reportValue
		if err := json.Unmarshal([]byte(response.GetReportJson()), &value); err != nil {
			t.Fatal(err)
		}
		if value.EventID != expected.id || value.MemberRewardsCents != expected.reward || value.Margin.ValueUSD != expected.margin || value.Margin.ValueKind != "modeled_estimate" {
			t.Fatalf("recorded report %s lacks sourced reward or modeled margin", expected.name)
		}
	}
	partner := new(gridosv1.GetEventReportResponse)
	readPlanningFixture(t, root, "ReportService/GetEventReport.partner.json", partner)
	if strings.Contains(strings.ToLower(partner.GetReportJson()), "site_id") {
		t.Fatal("partner report exposes site_id")
	}
	comparison := new(gridosv1.CompareEventReportsResponse)
	readPlanningFixture(t, root, "ReportService/CompareEventReports.json", comparison)
	changes := make(map[string]string)
	for _, difference := range comparison.GetDifferences() {
		changes[difference.GetField()] = difference.GetBefore() + ":" + difference.GetAfter()
	}
	if len(changes) != 2 || changes["member_rewards_cents"] != "725:1250" || changes["margin.value_usd"] != "-3.25:1.5" {
		t.Fatal("recorded comparison does not match published report values")
	}
}

func TestFixturesCaptureMemberOperatingStates(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "fixtures", "api")
	states := map[string]gridosv1.FleetOperatingState{
		"on_grid":                      gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_ON_GRID,
		"off_grid_outage":              gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OUTAGE,
		"off_grid_no_home_power":       gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_NO_HOME_POWER,
		"off_grid_overcurrent":         gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT,
		"off_grid_overcurrent_standby": gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT_STANDBY,
		"telemetry_unavailable":        gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_TELEMETRY_UNAVAILABLE,
	}
	for name, state := range states {
		response := new(gridosv1.GetMemberStatusResponse)
		readPlanningFixture(t, root, "MemberService/GetMemberStatus."+name+".json", response)
		if response.GetMemberId() == "" || response.GetSiteId() == "" || response.GetOperatingState() != state {
			t.Fatalf("recorded member state %s does not match request", name)
		}
	}
}

func readPlanningFixture(t *testing.T, root, name string, message proto.Message) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(content, message); err != nil {
		t.Fatal(err)
	}
}

func TestFixturesMatchContract(t *testing.T) {
	root := repositoryRoot(t)
	files := contractFiles(t, root)
	fixtureRoot := filepath.Join(root, "testdata", "fixtures", "api")
	indexContent, err := os.ReadFile(filepath.Join(fixtureRoot, "INDEX.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index fixtureIndex
	if err := json.Unmarshal(indexContent, &index); err != nil {
		t.Fatal(err)
	}
	mapped := make(map[string]bool)
	for screen, methods := range index.Screens {
		if len(methods) == 0 {
			t.Errorf("screen %q has no methods", screen)
		}
		for _, methodName := range methods {
			fixture := validateMethodFixture(t, files, fixtureRoot, methodName, "", fixtureVariant{})
			mapped[fixture] = true
		}
	}
	for methodName, variants := range index.Variants {
		service, method, found := strings.Cut(strings.TrimPrefix(methodName, "gridos.v1."), ".")
		if !found || !mapped[filepath.Join(service, method+".json")] {
			t.Errorf("variants for %s require a mapped default fixture", methodName)
		}
		for name, request := range variants {
			fixture := validateMethodFixture(t, files, fixtureRoot, methodName, name, request)
			mapped[fixture] = true
		}
	}
	assertEveryFixtureMapped(t, fixtureRoot, mapped)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "contracts", "buf.yaml")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository root not found")
		}
		directory = parent
	}
}

func contractFiles(t *testing.T, root string) *protoregistry.Files {
	t.Helper()
	descriptorPath := filepath.Join(t.TempDir(), "contracts.binpb")
	command := exec.Command("buf", "build", filepath.Join(root, "contracts"), "--as-file-descriptor-set", "-o", descriptorPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("buf build: %s: %v", output, err)
	}
	content, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(content, set); err != nil {
		t.Fatal(err)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func validateMethodFixture(t *testing.T, files *protoregistry.Files, fixtureRoot, methodName, variant string, request fixtureVariant) string {
	t.Helper()
	descriptor, err := files.FindDescriptorByName(protoreflect.FullName(methodName))
	if err != nil {
		t.Errorf("method %q is absent from descriptors: %v", methodName, err)
		return ""
	}
	method, ok := descriptor.(protoreflect.MethodDescriptor)
	if !ok {
		t.Errorf("%q is not a method", methodName)
		return ""
	}
	service := string(method.Parent().Name())
	name := string(method.Name())
	if variant != "" {
		if strings.ContainsAny(variant, "/\\.") || request.Role == "" || len(request.Body) == 0 {
			t.Errorf("invalid fixture variant %s.%s", methodName, variant)
			return ""
		}
		input := dynamicpb.NewMessage(method.Input())
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(request.Body, input); err != nil {
			t.Errorf("invalid fixture request %s.%s: %v", methodName, variant, err)
			return ""
		}
		name += "." + variant
	}
	fixture := filepath.Join(service, name+".json")
	content, err := os.ReadFile(filepath.Join(fixtureRoot, fixture))
	if err != nil {
		t.Errorf("%s: %v", fixture, err)
		return fixture
	}
	message := dynamicpb.NewMessage(method.Output())
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(content, message); err != nil {
		t.Errorf("%s: %v", fixture, err)
		return fixture
	}
	validateFixtureMetadata(t, fixture, message.ProtoReflect())
	return fixture
}

func validateFixtureMetadata(t *testing.T, fixture string, message protoreflect.Message) {
	t.Helper()
	fullName := message.Descriptor().FullName()
	if fullName == "gridos.v1.AggregateMetadata" {
		validateAggregateMetadata(t, fixture, message)
	}
	if fullName == "gridos.v1.DispatchEvent" || fullName == "gridos.v1.Site" || fullName == "gridos.v1.Device" {
		validateSimulatedRecord(t, fixture, message)
	}
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsList() && field.Message() != nil {
			list := value.List()
			for index := 0; index < list.Len(); index++ {
				validateFixtureMetadata(t, fixture, list.Get(index).Message())
			}
			return true
		}
		if field.Message() != nil {
			validateFixtureMetadata(t, fixture, value.Message())
		}
		return true
	})
}

func validateAggregateMetadata(t *testing.T, fixture string, message protoreflect.Message) {
	t.Helper()
	fields := message.Descriptor().Fields()
	for _, name := range []protoreflect.Name{"timestamp", "freshness", "provenance_mix"} {
		field := fields.ByName(name)
		if !message.Has(field) {
			t.Errorf("%s aggregate metadata lacks %s", fixture, name)
		}
	}
	provenanceMix := message.Get(fields.ByName("provenance_mix")).List()
	for index := 0; index < provenanceMix.Len(); index++ {
		entry := provenanceMix.Get(index).Message()
		provenance := entry.Descriptor().Fields().ByName("provenance")
		if entry.Get(provenance).Enum() != 5 {
			t.Errorf("%s aggregate provenance is not SIMULATED", fixture)
		}
	}
}

func validateSimulatedRecord(t *testing.T, fixture string, message protoreflect.Message) {
	t.Helper()
	field := message.Descriptor().Fields().ByName("provenance")
	if !message.Has(field) {
		t.Errorf("%s %s lacks provenance", fixture, message.Descriptor().Name())
		return
	}
	provenance := message.Get(field).Message()
	value := provenance.Descriptor().Fields().ByName("provenance")
	if provenance.Get(value).Enum() != 5 {
		t.Errorf("%s %s provenance is not SIMULATED", fixture, message.Descriptor().Name())
	}
}

func assertEveryFixtureMapped(t *testing.T, fixtureRoot string, mapped map[string]bool) {
	t.Helper()
	err := filepath.WalkDir(fixtureRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "INDEX.json" || filepath.Ext(path) != ".json" {
			return nil
		}
		relative, err := filepath.Rel(fixtureRoot, path)
		if err != nil {
			return err
		}
		if !mapped[relative] {
			t.Errorf("fixture %s is not mapped to a screen", strings.TrimSuffix(relative, ".json"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
