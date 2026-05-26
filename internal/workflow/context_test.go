package workflow

import (
	"encoding/json"
	"testing"
)

func TestCollectContext(t *testing.T) {
	stepResults := map[string]json.RawMessage{
		"read-task":       json.RawMessage(`{"response":"task analysis"}`),
		"build-callgraph": json.RawMessage(`{"callgraph":[]}`),
	}

	t.Run("empty deps returns nil", func(t *testing.T) {
		result := collectContext(nil, stepResults)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("deps resolved from stepResults", func(t *testing.T) {
		deps := []string{"read-task", "build-callgraph"}
		result := collectContext(deps, stepResults)
		if len(result) != 2 {
			t.Errorf("expected 2 context entries, got %d: %v", len(result), result)
		}
	})

	t.Run("missing dep skipped, found dep returned", func(t *testing.T) {
		deps := []string{"read-task", "unknown-step"}
		result := collectContext(deps, stepResults)
		if len(result) != 1 {
			t.Errorf("expected 1 entry, got %d: %v", len(result), result)
		}
		if _, ok := result["read-task"]; !ok {
			t.Error("read-task not in result")
		}
	})

	t.Run("all deps missing returns empty map not nil", func(t *testing.T) {
		deps := []string{"unknown-1", "unknown-2"}
		result := collectContext(deps, stepResults)
		// empty map: buildTaskWithContext will see len==0 and skip context
		if result == nil {
			t.Error("expected empty map, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}
