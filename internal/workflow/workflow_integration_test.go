package workflow_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/leokiin/mcp-conveer/internal/activity"
	"github.com/leokiin/mcp-conveer/internal/config"
	"github.com/leokiin/mcp-conveer/internal/workflow"
)

// Ensure activity import is used (OnActivity references activity.Registry).
var _ = activity.Registry{}

// SequentialWorkflowTestSuite tests SequentialWorkflow using the Temporal test environment.
type SequentialWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestSequentialWorkflowSuite(t *testing.T) {
	suite.Run(t, new(SequentialWorkflowTestSuite))
}

func (s *SequentialWorkflowTestSuite) Test_SingleStep() {
	env := s.NewTestWorkflowEnvironment()
	result := json.RawMessage(`{"summary":"ok"}`)

	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(result, nil)

	cfg := &config.Config{
		Workflow: []config.WorkflowStep{{Agent: "scanner"}},
		Agents:   []config.AgentDef{{Name: "scanner", Model: "ollama/llama3"}},
	}

	env.ExecuteWorkflow(workflow.SequentialWorkflow, workflow.SequentialInput{
		Config: cfg,
		Task:   "scan me",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var out workflow.SequentialResult
	s.NoError(env.GetWorkflowResult(&out))
	s.Len(out.Steps, 1)
	s.Empty(out.Steps[0].Error)
}

func (s *SequentialWorkflowTestSuite) Test_DependsOn() {
	env := s.NewTestWorkflowEnvironment()
	result := json.RawMessage(`{"done":true}`)

	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(result, nil).Times(2)

	cfg := &config.Config{
		Workflow: []config.WorkflowStep{
			{Agent: "reader"},
			{Agent: "reviewer", DependsOn: []string{"reader"}},
		},
		Agents: []config.AgentDef{
			{Name: "reader", Model: "ollama/llama3"},
			{Name: "reviewer", Model: "ollama/llama3"},
		},
	}

	env.ExecuteWorkflow(workflow.SequentialWorkflow, workflow.SequentialInput{
		Config: cfg,
		Task:   "review this",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var out workflow.SequentialResult
	s.NoError(env.GetWorkflowResult(&out))
	s.Len(out.Steps, 2)
}

func (s *SequentialWorkflowTestSuite) Test_NilConfigReturnsError() {
	env := s.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(workflow.SequentialWorkflow, workflow.SequentialInput{
		Config: nil,
		Task:   "task",
	})

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
}

func (s *SequentialWorkflowTestSuite) Test_ActivityError_CapturedInStep() {
	env := s.NewTestWorkflowEnvironment()

	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(json.RawMessage(nil), &mockErr{"agent down"})

	cfg := &config.Config{
		Workflow: []config.WorkflowStep{{Agent: "broken"}},
		Agents:   []config.AgentDef{{Name: "broken", Model: "ollama/llama3"}},
	}

	env.ExecuteWorkflow(workflow.SequentialWorkflow, workflow.SequentialInput{
		Config: cfg,
		Task:   "task",
	})

	s.True(env.IsWorkflowCompleted())

	var out workflow.SequentialResult
	s.NoError(env.GetWorkflowResult(&out))
	s.Len(out.Steps, 1)
	s.NotEmpty(out.Steps[0].Error)
}

// RoutedWorkflowTestSuite tests RoutedWorkflow using the Temporal test environment.
type RoutedWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestRoutedWorkflowSuite(t *testing.T) {
	suite.Run(t, new(RoutedWorkflowTestSuite))
}

func (s *RoutedWorkflowTestSuite) Test_RoutedWorkflow_Basic() {
	env := s.NewTestWorkflowEnvironment()

	// Sequential mocks: routing → scanner → synthesis (consumed in call order)
	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(json.RawMessage(`["scanner"]`), nil).Once()
	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(json.RawMessage(`{"issues":[]}`), nil).Once()
	env.OnActivity((*activity.Registry).CallAgent, mock.Anything, mock.Anything, mock.Anything).
		Return(json.RawMessage(`{"response":"all good"}`), nil).Once()

	cfg := &config.Config{
		TeamLead: config.TeamLead{Model: "ollama/llama3"},
		Agents:   []config.AgentDef{{Name: "scanner", Model: "ollama/llama3"}},
	}

	env.ExecuteWorkflow(workflow.RoutedWorkflow, workflow.RoutedInput{
		Config: cfg,
		Task:   "scan the repo",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var out workflow.RoutedResult
	s.NoError(env.GetWorkflowResult(&out))
	s.Contains(out.SelectedAgents, "scanner")
}
