package contract_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCommandIntentCanonicalJSON(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "fixtures", "contracts", "command_intent.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, fixture, "", "  "); err != nil {
		t.Fatal(err)
	}
	fixture = formatted.Bytes()
	message := &gridosv1.CommandIntent{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(fixture, message); err != nil {
		t.Fatal(err)
	}
	canonical, err := (protojson.MarshalOptions{}).Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(fixture) || !json.Valid(canonical) {
		t.Fatal("fixture or generated command intent is not valid JSON")
	}
	var expected, actual bytes.Buffer
	if err := json.Compact(&expected, fixture); err != nil {
		t.Fatal(err)
	}
	if err := json.Compact(&actual, canonical); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected.Bytes(), actual.Bytes()) {
		t.Fatalf("canonical JSON differs\nfixture: %s\nactual:  %s", fixture, canonical)
	}
}
