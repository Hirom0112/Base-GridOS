package dispatch

import (
	"io"
	"log/slog"
	"os"
	"testing"

	historypb "go.temporal.io/api/history/v1"
	temporallog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestReplay(t *testing.T) {
	contents, err := os.ReadFile("testdata/workflow_history.json")
	if err != nil {
		t.Fatal(err)
	}
	history := new(historypb.History)
	if err = protojson.Unmarshal(contents, history); err != nil {
		t.Fatal(err)
	}
	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflow(Workflow)
	logger := temporallog.NewStructuredLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err = replayer.ReplayWorkflowHistory(logger, history); err != nil {
		t.Fatal(err)
	}
}
