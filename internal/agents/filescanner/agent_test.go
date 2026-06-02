package filescanner_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/agents/filescanner"
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

const validScanJSON = `{
	"file": "main.go",
	"issues": [{"severity": "warning", "line": 10, "message": "unused var"}],
	"summary": "one warning"
}`

func TestHandle_FileNotFound(t *testing.T) {
	a := filescanner.New(&fakeProvider{response: validScanJSON})
	_, err := a.Handle(context.Background(), "/nonexistent/path/file.go")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "read file")
}

func TestHandle_ValidScan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(path, []byte("package main"), 0600))

	a := filescanner.New(&fakeProvider{response: validScanJSON})
	result, err := a.Handle(context.Background(), path)
	require.NoError(t, err)

	var got filescanner.ScanResult
	require.NoError(t, json.Unmarshal(result, &got))
	assert.Equal(t, "main.go", got.File)
	assert.Len(t, got.Issues, 1)
	assert.Equal(t, "one warning", got.Summary)
}

func TestHandle_LLMReturnsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.go")
	require.NoError(t, os.WriteFile(path, []byte("package main"), 0600))

	a := filescanner.New(&fakeProvider{response: "not valid json at all"})
	_, err := a.Handle(context.Background(), path)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "parse llm response")
}
