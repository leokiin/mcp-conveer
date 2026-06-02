package generic_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/agents/generic"
	"github.com/leokiin/mcp-conveer/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	response string
	err      error
}

func (f *fakeProvider) Complete(_ context.Context, _, _ string) (string, provider.Usage, error) {
	return f.response, provider.Usage{}, f.err
}

func TestHandle_ValidJSONPassthrough(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: `{"key":"value"}`})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(result, &got))
	assert.Equal(t, `"value"`, string(got["key"]))
	assert.Contains(t, string(result), "_tokens")
}

func TestHandle_PlainTextWrapped(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: "plain text response"})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	assert.Contains(t, string(result), `"response"`)
	assert.Contains(t, string(result), "plain text response")
	assert.Contains(t, string(result), "_tokens")
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
	assert.Contains(t, string(result), "_tokens")
}

func TestHandle_JSONArray(t *testing.T) {
	a := generic.New("test", "role", &fakeProvider{response: `["a","b"]`})
	result, err := a.Handle(context.Background(), "task")
	require.NoError(t, err)
	// arrays can't be merged, wrapped as response field
	assert.Contains(t, string(result), `"response"`)
	assert.Contains(t, string(result), "_tokens")
}
