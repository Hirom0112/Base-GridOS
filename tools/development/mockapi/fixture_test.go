package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type fixtureIndex struct {
	Screens map[string][]string `json:"screens"`
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
			fixture := validateMethodFixture(t, files, fixtureRoot, methodName)
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

func validateMethodFixture(t *testing.T, files *protoregistry.Files, fixtureRoot, methodName string) string {
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
	fixture := filepath.Join(service, string(method.Name())+".json")
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
