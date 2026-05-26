package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/agent"
)

type mockAgent struct {
	name   string
	result json.RawMessage
	err    error
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Handle(_ context.Context, _ string) (json.RawMessage, error) {
	return m.result, m.err
}

func TestMCPServer_Call(t *testing.T) {
	expected := json.RawMessage(`{"status":"ok","issues":[]}`)

	srv := agent.NewMCPServer(&mockAgent{
		name:   "test-agent",
		result: expected,
	})

	got, err := srv.Call(context.Background(), "analyze this")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got) != string(expected) {
		t.Errorf("got %s, want %s", got, expected)
	}
}

func TestMCPServer_CallPropagatesError(t *testing.T) {
	import_err := json.RawMessage(nil)
	srv := agent.NewMCPServer(&mockAgent{
		name:   "failing-agent",
		result: import_err,
		err:    &mockError{"agent failed"},
	})

	_, err := srv.Call(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "agent failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }
