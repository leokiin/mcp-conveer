package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/leokiin/mcp-conveer/internal/activity"
	"github.com/leokiin/mcp-conveer/internal/config"
)

// SequentialInput is the input to SequentialWorkflow.
type SequentialInput struct {
	Config *config.Config `json:"config"`
	Task   string         `json:"task"`
}

// StepOutput captures the result of a single workflow step.
type StepOutput struct {
	StepName     string                     `json:"step_name"`
	Result       json.RawMessage            `json:"result,omitempty"`
	Error        string                     `json:"error,omitempty"`
	InnerResults map[string]json.RawMessage `json:"inner_results,omitempty"`
}

// SequentialResult holds all step outputs.
type SequentialResult struct {
	Steps []StepOutput `json:"steps"`
}

// SequentialWorkflow executes a sequential (graph-based) conveer pipeline.
// Steps without depends_on run in parallel; depends_on creates ordering constraints.
// Loop steps repeat until the condition is met or max_iterations is exhausted.
func SequentialWorkflow(ctx workflow.Context, input SequentialInput) (SequentialResult, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	if input.Config == nil {
		return SequentialResult{}, fmt.Errorf("sequential workflow: config is required")
	}
	layers, err := config.BuildLayers(input.Config.Workflow)
	if err != nil {
		return SequentialResult{}, fmt.Errorf("build layers: %w", err)
	}

	// stepResults accumulates outputs by step name for condition evaluation.
	stepResults := make(map[string]json.RawMessage)
	var allSteps []StepOutput

	for _, layer := range layers {
		layerOutputs, err := runLayer(ctx, layer, input.Task, stepResults, input.Config)
		if err != nil {
			return SequentialResult{}, err
		}
		for _, out := range layerOutputs {
			stepResults[out.StepName] = out.Result
			for k, v := range out.InnerResults {
				stepResults[k] = v
			}
			allSteps = append(allSteps, out)
		}
	}

	return SequentialResult{Steps: allSteps}, nil
}

// runLayer executes all steps in a layer in parallel and collects results.
func runLayer(
	ctx workflow.Context,
	layer config.Layer,
	task string,
	stepResults map[string]json.RawMessage,
	cfg *config.Config,
) ([]StepOutput, error) {
	type indexedOutput struct {
		index  int
		output StepOutput
	}

	ch := workflow.NewChannel(ctx)
	outputs := make([]StepOutput, len(layer))

	for i, step := range layer {
		i, step := i, step
		workflow.Go(ctx, func(ctx workflow.Context) {
			out := runStep(ctx, step, task, stepResults, cfg)
			ch.Send(ctx, indexedOutput{index: i, output: out})
		})
	}

	for range layer {
		var io indexedOutput
		ch.Receive(ctx, &io)
		outputs[io.index] = io.output
	}

	return outputs, nil
}

// runStep executes a single step, handling loop steps recursively.
func runStep(
	ctx workflow.Context,
	step config.WorkflowStep,
	task string,
	stepResults map[string]json.RawMessage,
	cfg *config.Config,
) StepOutput {
	name := config.StepName(step)

	if step.Loop != nil {
		return runLoopStep(ctx, name, step.Loop, task, stepResults, cfg)
	}

	stepContext := collectContext(step.DependsOn, stepResults)

	callInput := activity.CallInput{
		AgentName: step.Agent,
		Task:      task,
		Context:   stepContext,
	}
	if def := cfg.AgentByName(step.Agent); def != nil && def.InputStep != "" {
		callInput.InputStep = def.InputStep
		callInput.InputField = def.InputField
	}

	var result json.RawMessage
	err := workflow.ExecuteActivity(ctx, (*activity.Registry).CallAgent, callInput).Get(ctx, &result)

	if err != nil {
		return StepOutput{StepName: name, Error: err.Error()}
	}
	return StepOutput{StepName: name, Result: result}
}

// runLoopStep executes a loop block, repeating until the until condition is true
// or max_iterations is reached.
func runLoopStep(
	ctx workflow.Context,
	name string,
	loop *config.Loop,
	task string,
	stepResults map[string]json.RawMessage,
	cfg *config.Config,
) StepOutput {
	localResults := copyMap(stepResults)
	var lastOutput StepOutput

	outerKeys := make(map[string]bool, len(stepResults))
	outerSteps := make([]string, 0, len(stepResults))
	for k := range stepResults {
		outerKeys[k] = true
		outerSteps = append(outerSteps, k)
	}

	if loop.MaxIterations <= 0 {
		return StepOutput{StepName: name, Error: "loop max_iterations must be > 0"}
	}

	for i := 0; i < loop.MaxIterations; i++ {
		layers, err := config.BuildLayers(loop.Steps, outerSteps...)
		if err != nil {
			return StepOutput{StepName: name, Error: err.Error()}
		}

		for _, layer := range layers {
			outs, err := runLayer(ctx, layer, task, localResults, cfg)
			if err != nil {
				return StepOutput{StepName: name, Error: err.Error()}
			}
			for _, o := range outs {
				localResults[o.StepName] = o.Result
				lastOutput = o
			}
		}

		if evalUntil(loop.Until, localResults) {
			break
		}
	}

	// Expose final inner step results so post-loop steps can reference them.
	innerResults := make(map[string]json.RawMessage)
	for k, v := range localResults {
		if !outerKeys[k] {
			innerResults[k] = v
		}
	}

	return StepOutput{StepName: name, Result: lastOutput.Result, InnerResults: innerResults}
}

// evalUntil checks a simple "step.field == value" condition against step results.
func evalUntil(condition string, results map[string]json.RawMessage) bool {
	if condition == "" {
		return true
	}

	// Parse "step.field == value"
	parts := strings.SplitN(condition, " == ", 2)
	if len(parts) != 2 {
		return false
	}

	lhs := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])

	// Split "step.field"
	dot := strings.IndexByte(lhs, '.')
	if dot < 0 {
		return false
	}
	stepName := lhs[:dot]
	field := lhs[dot+1:]

	raw, ok := results[stepName]
	if !ok || raw == nil {
		return false
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}

	val, ok := obj[field]
	if !ok {
		return false
	}

	// Compare as JSON-marshaled strings to handle bool/string/number.
	gotJSON, err := json.Marshal(val)
	if err != nil {
		return false
	}
	wantJSON, err := json.Marshal(parseValue(rhs))
	if err != nil {
		return false
	}

	return string(gotJSON) == string(wantJSON)
}

// parseValue converts a YAML/JSON-like string literal to a Go value.
func parseValue(s string) interface{} {
	switch s {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	var n float64
	if _, err := fmt.Sscanf(s, "%f", &n); err == nil {
		return n
	}
	return strings.Trim(s, `"'`)
}

// collectContext builds a context map from prior step results for the given depends_on list.
func collectContext(deps []string, stepResults map[string]json.RawMessage) map[string]json.RawMessage {
	if len(deps) == 0 {
		return nil
	}
	ctx := make(map[string]json.RawMessage, len(deps))
	for _, dep := range deps {
		if result, ok := stepResults[dep]; ok {
			ctx[dep] = result
		}
	}
	return ctx
}

func copyMap(m map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
