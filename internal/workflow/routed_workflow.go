package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/leokiin/mcp-conveer/internal/activity"
	"github.com/leokiin/mcp-conveer/internal/config"
)

// RoutedInput is the input to RoutedWorkflow.
type RoutedInput struct {
	Config *config.Config `json:"config"`
	Task   string         `json:"task"`
}

// RoutedResult holds the teamlead's final synthesis.
type RoutedResult struct {
	SelectedAgents []string                   `json:"selected_agents"`
	AgentOutputs   map[string]json.RawMessage `json:"agent_outputs"`
	Summary        string                     `json:"summary"`
}

const routingPromptTemplate = `You are a teamlead orchestrating a team of specialized agents.

Available agents:
%s

Task: %s

Respond with a JSON array of agent names to call, e.g. ["scanner", "security"].
Return only valid JSON, no markdown.`

// RoutedWorkflow lets the teamlead LLM decide which agents to invoke for the task.
// The teamlead first selects agents, then they run in parallel, then the teamlead synthesizes.
func RoutedWorkflow(ctx workflow.Context, input RoutedInput) (RoutedResult, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	// Step 1: teamlead selects agents.
	agentList := buildAgentList(input.Config.Agents)
	routingTask := fmt.Sprintf(routingPromptTemplate, agentList, input.Task)

	var routingResult json.RawMessage
	err := workflow.ExecuteActivity(ctx, (*activity.Registry).CallAgent, activity.CallInput{
		AgentName: "teamlead",
		Task:      routingTask,
	}).Get(ctx, &routingResult)
	if err != nil {
		return RoutedResult{}, fmt.Errorf("teamlead routing: %w", err)
	}

	var selected []string
	if err := json.Unmarshal(routingResult, &selected); err != nil {
		// LLM may wrap a JSON array in {"response":"[...]"} — try to unwrap.
		var wrapper struct {
			Response string `json:"response"`
		}
		if wErr := json.Unmarshal(routingResult, &wrapper); wErr == nil && wrapper.Response != "" {
			if err2 := json.Unmarshal([]byte(wrapper.Response), &selected); err2 != nil {
				return RoutedResult{}, fmt.Errorf("parse agent selection %q: %w", string(routingResult), err2)
			}
		} else {
			return RoutedResult{}, fmt.Errorf("parse agent selection %q: %w", string(routingResult), err)
		}
	}

	// Step 2: run selected agents in parallel.
	type namedResult struct {
		name string
		out  json.RawMessage
		err  string
	}

	ch := workflow.NewChannel(ctx)
	for _, name := range selected {
		name := name
		workflow.Go(ctx, func(ctx workflow.Context) {
			var result json.RawMessage
			err := workflow.ExecuteActivity(ctx, (*activity.Registry).CallAgent, activity.CallInput{
				AgentName: name,
				Task:      input.Task,
			}).Get(ctx, &result)

			nr := namedResult{name: name, out: result}
			if err != nil {
				nr.err = err.Error()
			}
			ch.Send(ctx, nr)
		})
	}

	agentOutputs := make(map[string]json.RawMessage, len(selected))
	for range selected {
		var nr namedResult
		ch.Receive(ctx, &nr)
		if nr.err != "" {
			errJSON, _ := json.Marshal(map[string]string{"error": nr.err})
			agentOutputs[nr.name] = errJSON
		} else {
			agentOutputs[nr.name] = nr.out
		}
	}

	// Step 3: teamlead synthesizes results.
	// Sort keys for deterministic replay — Temporal rejects non-deterministic history.
	agentNames := make([]string, 0, len(agentOutputs))
	for name := range agentOutputs {
		agentNames = append(agentNames, name)
	}
	sort.Strings(agentNames)

	synthParts := []string{"Results from agents:", ""}
	for _, name := range agentNames {
		synthParts = append(synthParts, fmt.Sprintf("[%s]: %s", name, string(agentOutputs[name])))
	}
	synthParts = append(synthParts, "", "Please provide a concise final summary.")

	var summaryResult json.RawMessage
	if err := workflow.ExecuteActivity(ctx, (*activity.Registry).CallAgent, activity.CallInput{
		AgentName: "teamlead",
		Task:      strings.Join(synthParts, "\n"),
	}).Get(ctx, &summaryResult); err != nil {
		return RoutedResult{}, fmt.Errorf("teamlead synthesis: %w", err)
	}

	return RoutedResult{
		SelectedAgents: selected,
		AgentOutputs:   agentOutputs,
		Summary:        string(summaryResult),
	}, nil
}

func buildAgentList(agents []config.AgentDef) string {
	var b strings.Builder
	for _, a := range agents {
		b.WriteString("- ")
		b.WriteString(a.Name)
		if a.Role != "" {
			b.WriteString(": ")
			b.WriteString(a.Role)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
