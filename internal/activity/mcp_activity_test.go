package activity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/agent"
)

func TestExtractField(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		field string
		want  string
	}{
		{"empty field returns raw JSON", `{"k":"v"}`, "", `{"k":"v"}`},
		{"string field extracted", `{"k":"gin"}`, "k", "gin"},
		{"numeric field falls back to raw JSON", `{"k":42}`, "k", `{"k":42}`},
		{"not found field falls back to raw JSON", `{"k":"v"}`, "missing", `{"k":"v"}`},
		{"invalid JSON falls back to raw string", `not json`, "k", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractField(json.RawMessage(tc.raw), tc.field)
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// mockAgent is a minimal Agent implementation for tests.
type mockAgent struct {
	name   string
	result json.RawMessage
	err    error
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Handle(_ context.Context, _ string) (json.RawMessage, error) {
	return m.result, m.err
}

func TestCallAgent_UnknownAgent(t *testing.T) {
	r := NewRegistry()
	_, err := r.CallAgent(context.Background(), CallInput{AgentName: "missing"})
	if err == nil {
		t.Fatal("expected error")
	}
	var uae *UnknownAgentError
	if !errors.As(err, &uae) {
		t.Fatalf("want *UnknownAgentError, got %T: %v", err, err)
	}
	if uae.Name != "missing" {
		t.Errorf("want Name 'missing', got %q", uae.Name)
	}
}

func TestCallAgent_Success(t *testing.T) {
	r := NewRegistry()
	r.Register("echo", agent.NewMCPServer(&mockAgent{
		name:   "echo",
		result: json.RawMessage(`{"ok":true}`),
	}))

	got, err := r.CallAgent(context.Background(), CallInput{AgentName: "echo", Task: "ping"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("got %s, want {\"ok\":true}", got)
	}
}

func TestCallAgent_InputStep(t *testing.T) {
	r := NewRegistry()
	var received string
	r.Register("echo", agent.NewMCPServer(&mockAgent{
		name: "echo",
		result: json.RawMessage(`{"response":"pong"}`),
	}))
	_ = received

	got, err := r.CallAgent(context.Background(), CallInput{
		AgentName: "echo",
		Task:      "ignored",
		Context:   map[string]json.RawMessage{"prev": json.RawMessage(`{"lib":"gin"}`)},
		InputStep: "prev",
		InputField: "lib",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = got
}

func TestUnknownAgentError_Is(t *testing.T) {
	err := &UnknownAgentError{"scanner"}

	// matches same name
	if !errors.Is(err, &UnknownAgentError{"scanner"}) {
		t.Error("expected Is match for same name")
	}
	// matches wildcard (empty name)
	if !errors.Is(err, &UnknownAgentError{}) {
		t.Error("expected Is match for empty-name wildcard")
	}
	// does not match different name
	if errors.Is(err, &UnknownAgentError{"other"}) {
		t.Error("unexpected Is match for different name")
	}
}

func TestBuildTaskWithContext(t *testing.T) {
	task := "find why auth/token.go panics"

	t.Run("no context returns original task", func(t *testing.T) {
		result := buildTaskWithContext(task, nil)
		if result != task {
			t.Errorf("expected original task, got: %s", result)
		}
	})

	t.Run("context appended to task", func(t *testing.T) {
		ctx := map[string]json.RawMessage{
			"read-task": json.RawMessage(`{"response":"nil pointer issue"}`),
		}
		result := buildTaskWithContext(task, ctx)
		if result == task {
			t.Error("context was not appended")
		}
		t.Logf("enriched task:\n%s", result)
	})

	t.Run("multiple context steps", func(t *testing.T) {
		ctx := map[string]json.RawMessage{
			"read-task":       json.RawMessage(`{"response":"task analysis"}`),
			"build-callgraph": json.RawMessage(`{"callgraph":[{"caller":"A","callee":"B"}]}`),
		}
		result := buildTaskWithContext(task, ctx)
		if result == task {
			t.Error("context was not appended")
		}
		t.Logf("enriched task:\n%s", result)
	})
}

