package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvalUntil_EmptyCondition(t *testing.T) {
	assert.True(t, evalUntil("", nil))
}

func TestEvalUntil_BoolTrue(t *testing.T) {
	results := map[string]json.RawMessage{
		"reviewer": json.RawMessage(`{"approved": true}`),
	}
	assert.True(t, evalUntil("reviewer.approved == true", results))
}

func TestEvalUntil_BoolFalse(t *testing.T) {
	results := map[string]json.RawMessage{
		"reviewer": json.RawMessage(`{"approved": false}`),
	}
	assert.False(t, evalUntil("reviewer.approved == true", results))
}

func TestEvalUntil_StringMatch(t *testing.T) {
	results := map[string]json.RawMessage{
		"checker": json.RawMessage(`{"status": "done"}`),
	}
	assert.True(t, evalUntil(`checker.status == "done"`, results))
}

func TestEvalUntil_StringNoMatch(t *testing.T) {
	results := map[string]json.RawMessage{
		"checker": json.RawMessage(`{"status": "pending"}`),
	}
	assert.False(t, evalUntil(`checker.status == "done"`, results))
}

func TestEvalUntil_NumberMatch(t *testing.T) {
	results := map[string]json.RawMessage{
		"counter": json.RawMessage(`{"count": 3}`),
	}
	assert.True(t, evalUntil("counter.count == 3", results))
}

func TestEvalUntil_MissingStep(t *testing.T) {
	results := map[string]json.RawMessage{}
	assert.False(t, evalUntil("missing.field == true", results))
}

func TestEvalUntil_MissingField(t *testing.T) {
	results := map[string]json.RawMessage{
		"step": json.RawMessage(`{"other": true}`),
	}
	assert.False(t, evalUntil("step.approved == true", results))
}

func TestEvalUntil_InvalidJSON(t *testing.T) {
	results := map[string]json.RawMessage{
		"step": json.RawMessage(`not json`),
	}
	assert.False(t, evalUntil("step.field == true", results))
}

func TestEvalUntil_MalformedCondition(t *testing.T) {
	results := map[string]json.RawMessage{
		"step": json.RawMessage(`{"field": true}`),
	}
	assert.False(t, evalUntil("nodot", results))
	assert.False(t, evalUntil("step.field", results))
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		input string
		want  interface{}
	}{
		{"true", true},
		{"false", false},
		{"null", nil},
		{"42", float64(42)},
		{"3.14", float64(3.14)},
		{`"hello"`, "hello"},
		{"hello", "hello"},
	}
	for _, tt := range tests {
		got := parseValue(tt.input)
		assert.Equal(t, tt.want, got, "parseValue(%q)", tt.input)
	}
}
