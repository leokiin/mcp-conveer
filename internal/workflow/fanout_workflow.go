package workflow

import (
	"encoding/json"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/leokiin/mcp-conveer/internal/activity"
)

// FanOutInput is the input to the FanOutWorkflow.
type FanOutInput struct {
	AgentName string   `json:"agent_name"`
	Tasks     []string `json:"tasks"`
}

// FanOutResult holds the per-task results from a fan-out run.
type FanOutResult struct {
	Results []TaskResult `json:"results"`
}

// TaskResult pairs a task with the agent's JSON output (or error).
type TaskResult struct {
	Task   string          `json:"task"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// FanOutWorkflow runs AgentName in parallel for each task in Tasks.
// It waits for all activities to complete and aggregates their results.
func FanOutWorkflow(ctx workflow.Context, input FanOutInput) (FanOutResult, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	type indexedResult struct {
		index int
		tr    TaskResult
	}

	results := make([]TaskResult, len(input.Tasks))
	ch := workflow.NewChannel(ctx)

	for i, task := range input.Tasks {
		i, task := i, task
		workflow.Go(ctx, func(ctx workflow.Context) {
			var out json.RawMessage
			err := workflow.ExecuteActivity(ctx, (*activity.Registry).CallAgent, activity.CallInput{
				AgentName: input.AgentName,
				Task:      task,
			}).Get(ctx, &out)

			tr := TaskResult{Task: task}
			if err != nil {
				tr.Error = err.Error()
			} else {
				tr.Result = out
			}
			ch.Send(ctx, indexedResult{index: i, tr: tr})
		})
	}

	for range input.Tasks {
		var ir indexedResult
		ch.Receive(ctx, &ir)
		results[ir.index] = ir.tr
	}

	return FanOutResult{Results: results}, nil
}
