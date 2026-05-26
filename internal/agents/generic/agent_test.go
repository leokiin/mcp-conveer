package generic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/agents/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	response string
	err      error
}

func (f *fakeProvider) Complete(_ context.Context, _, _ string) (string, error) {
	return f.response, f.err
}

func TestHandle_ValidJSONPassthrough(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: `{"key":"value"}`})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	assert.JSONEq(t, `{"key":"value"}`, string(result))
}

func TestHandle_PlainTextWrapped(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: "plain text response"})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	assert.JSONEq(t, `{"response":"plain text response"}`, string(result))
}

func TestHandle_ProviderError(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{err: errors.New("api down")})
	_, err := a.Handle(context.Background(), "task")
	assert.ErrorContains(t, err, "api down")
}

func TestHandle_EmptyJSONObject(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: `{}`})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(result))
}

func TestHandle_JSONArray(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: `["a","b"]`})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	assert.JSONEq(t, `["a","b"]`, string(result))
}
