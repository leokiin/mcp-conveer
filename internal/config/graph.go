package config

import "fmt"

// Layer is a group of steps that can execute in parallel.
// Steps within a layer have no interdependencies.
type Layer []WorkflowStep

// BuildLayers performs a topological sort on workflow steps grouped by depends_on.
// Steps with no dependencies form the first layer; each subsequent layer contains
// steps whose dependencies are all satisfied by prior layers.
//
// outerResolved contains step names already completed in an outer scope (e.g. outside a loop).
// References to those names are treated as satisfied without validation.
//
// Returns an error if a cycle is detected or an unknown dependency is referenced.
func BuildLayers(steps []WorkflowStep, outerResolved ...string) ([]Layer, error) {
	// Map step names to their definitions.
	byName := make(map[string]WorkflowStep, len(steps))
	for _, s := range steps {
		name := StepName(s)
		byName[name] = s
	}

	// Validate all depends_on references exist (outer steps are exempt).
	outer := make(map[string]bool, len(outerResolved))
	for _, name := range outerResolved {
		outer[name] = true
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := byName[dep]; !ok && !outer[dep] {
				return nil, fmt.Errorf("step %q depends on unknown step %q", StepName(s), dep)
			}
		}
	}

	resolved := make(map[string]bool)
	for _, name := range outerResolved {
		resolved[name] = true
	}
	var layers []Layer

	remaining := make([]WorkflowStep, len(steps))
	copy(remaining, steps)

	for len(remaining) > 0 {
		var layer Layer
		var next []WorkflowStep

		for _, s := range remaining {
			if allResolved(s.DependsOn, resolved) {
				layer = append(layer, s)
			} else {
				next = append(next, s)
			}
		}

		if len(layer) == 0 {
			// No progress — cycle detected.
			names := make([]string, len(remaining))
			for i, s := range remaining {
				names[i] = StepName(s)
			}
			return nil, fmt.Errorf("cycle detected among steps: %v", names)
		}

		layers = append(layers, layer)
		for _, s := range layer {
			resolved[StepName(s)] = true
		}
		remaining = next
	}

	return layers, nil
}

// StepName returns the canonical name of a workflow step.
func StepName(s WorkflowStep) string {
	if s.Step != "" {
		return s.Step
	}
	return s.Agent
}

func allResolved(deps []string, resolved map[string]bool) bool {
	for _, d := range deps {
		if !resolved[d] {
			return false
		}
	}
	return true
}
