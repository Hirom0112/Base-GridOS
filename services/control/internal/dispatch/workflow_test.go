package dispatch

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestSmoke(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	environment := suite.NewTestWorkflowEnvironment()
	environment.ExecuteWorkflow(Workflow, Input{EventID: "event-1"})

	require.True(t, environment.IsWorkflowCompleted())
	require.NoError(t, environment.GetWorkflowError())
	require.Equal(t, "gridos-dispatch", TaskQueue)
}
