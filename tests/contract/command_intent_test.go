package contract_test

import (
	"bytes"
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
	message := &gridosv1.CommandIntent{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(fixture, message); err != nil {
		t.Fatal(err)
	}
	canonical, err := (protojson.MarshalOptions{}).Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(fixture, canonical) {
		t.Fatalf("canonical JSON differs\nfixture: %s\nactual:  %s", fixture, canonical)
	}
}
