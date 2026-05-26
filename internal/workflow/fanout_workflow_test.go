package workflow_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/leokiin/mcp-conveer/internal/activity"
	"github.com/leokiin/mcp-conveer/internal/workflow"
)

type FanOutWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestFanOutWorkflowSuite(t *testing.T) {
	suite.Run(t, new(FanOutWorkflowTestSuite))
}

func (s *FanOutWorkflowTestSuite) Test_FanOutWorkflow_AllTasksRun() {
	env := s.NewTestWorkflowEnvironment()

	tasks := []string{"file1.go", "file2.go", "file3.go"}
	result := json.RawMessage(`{"file":"x.go","issues":[],"summary":"ok"}`)

	// Match any receiver + any ctx + any input; return same result for each call.
	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(result, nil).Times(len(tasks))

	env.ExecuteWorkflow(workflow.FanOutWorkflow, workflow.FanOutInput{
		AgentName: "file-scanner",
		Tasks:     tasks,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var out workflow.FanOutResult
	s.NoError(env.GetWorkflowResult(&out))
	s.Len(out.Results, len(tasks))
	for _, r := range out.Results {
		s.Empty(r.Error)
		s.NotEmpty(r.Result)
	}
}

func (s *FanOutWorkflowTestSuite) Test_FanOutWorkflow_ActivityError() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(json.RawMessage(nil), &mockErr{"scanner down"})

	env.ExecuteWorkflow(workflow.FanOutWorkflow, workflow.FanOutInput{
		AgentName: "file-scanner",
		Tasks:     []string{"broken.go"},
	})

	s.True(env.IsWorkflowCompleted())

	var out workflow.FanOutResult
	s.NoError(env.GetWorkflowResult(&out))
	s.Len(out.Results, 1)
	s.NotEmpty(out.Results[0].Error)
}

type mockErr struct{ msg string }

func (e *mockErr) Error() string { return e.msg }
