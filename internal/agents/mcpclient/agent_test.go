package mcpclient

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNew_defaultInputKey(t *testing.T) {
	a := New("test", "echo", "my-tool", "", nil)
	if a.inputKey != "query" {
		t.Errorf("expected default inputKey 'query', got %q", a.inputKey)
	}
}

func TestNew_customInputKey(t *testing.T) {
	a := New("test", "echo", "my-tool", "prompt", nil)
	if a.inputKey != "prompt" {
		t.Errorf("expected inputKey 'prompt', got %q", a.inputKey)
	}
}

func TestHandle_emptyCommand(t *testing.T) {
	a := New("bad", "", "tool", "query", nil)
	_, err := a.Handle(context.Background(), "task")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{"npx -y pkg", []string{"npx", "-y", "pkg"}, false},
		{"npx -y 'pkg with spaces'", []string{"npx", "-y", "pkg with spaces"}, false},
		{`npx -y "pkg with spaces"`, []string{"npx", "-y", "pkg with spaces"}, false},
		{"npx\t-y\tpkg", []string{"npx", "-y", "pkg"}, false},
		{"", nil, false},
		{"'unterminated", nil, true},
		{`"unterminated`, nil, true},
	}
	for _, tc := range cases {
		got, err := splitCommand(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("splitCommand(%q): want error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitCommand(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("splitCommand(%q): got %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitCommand(%q)[%d]: got %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestExtractText_multipleContent(t *testing.T) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "hello"},
			mcp.TextContent{Type: "text", Text: "world"},
		},
	}
	got := extractText(result)
	want := "hello\nworld"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestExtractText_noTextContent(t *testing.T) {
	result := &mcp.CallToolResult{Content: []mcp.Content{}}
	got := extractText(result)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
