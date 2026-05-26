package config_test

import (
	"testing"

	"github.com/leokiin/mcp-conveer/internal/config"
)

func TestBuildLayers_Linear(t *testing.T) {
	steps := []config.WorkflowStep{
		{Step: "a"},
		{Step: "b", DependsOn: []string{"a"}},
		{Step: "c", DependsOn: []string{"b"}},
	}
	layers, err := config.BuildLayers(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Fatalf("want 3 layers, got %d", len(layers))
	}
	for i, l := range layers {
		if len(l) != 1 {
			t.Errorf("layer %d: want 1 step, got %d", i, len(l))
		}
	}
}

func TestBuildLayers_Parallel(t *testing.T) {
	steps := []config.WorkflowStep{
		{Agent: "scanner"},
		{Agent: "security"},
		{Agent: "performance"},
	}
	layers, err := config.BuildLayers(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 {
		t.Fatalf("want 1 layer (all parallel), got %d", len(layers))
	}
	if len(layers[0]) != 3 {
		t.Errorf("want 3 steps in layer 0, got %d", len(layers[0]))
	}
}

func TestBuildLayers_Mixed(t *testing.T) {
	// plan → [api-designer, scanner in parallel] → reviewer
	steps := []config.WorkflowStep{
		{Step: "plan"},
		{Step: "api-design", DependsOn: []string{"plan"}},
		{Step: "scan", DependsOn: []string{"plan"}},
		{Step: "review", DependsOn: []string{"api-design", "scan"}},
	}
	layers, err := config.BuildLayers(steps)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Fatalf("want 3 layers, got %d: %v", len(layers), layers)
	}
	if len(layers[1]) != 2 {
		t.Errorf("layer 1 should have 2 parallel steps, got %d", len(layers[1]))
	}
}

func TestBuildLayers_CycleDetected(t *testing.T) {
	steps := []config.WorkflowStep{
		{Step: "a", DependsOn: []string{"b"}},
		{Step: "b", DependsOn: []string{"a"}},
	}
	_, err := config.BuildLayers(steps)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

func TestBuildLayers_UnknownDep(t *testing.T) {
	steps := []config.WorkflowStep{
		{Step: "a", DependsOn: []string{"nonexistent"}},
	}
	_, err := config.BuildLayers(steps)
	if err == nil {
		t.Fatal("expected error for unknown dependency, got nil")
	}
}

func TestBuildLayers_LoopDepsFromOuter(t *testing.T) {
	// incident-review scenario: loop inner steps reference outer steps
	loopSteps := []config.WorkflowStep{
		{Agent: "incident-finder", DependsOn: []string{"build-callgraph", "read-task"}},
		{Agent: "problem-solver", DependsOn: []string{"incident-finder"}},
	}
	outer := []string{"read-task", "build-callgraph"}
	layers, err := config.BuildLayers(loopSteps, outer...)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("want 2 layers, got %d", len(layers))
	}
}
