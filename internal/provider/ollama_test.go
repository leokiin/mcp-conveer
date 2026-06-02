package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/provider"
)


func TestOllamaProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "llama3" {
			t.Errorf("want model llama3, got %s", req.Model)
		}

		// Expect system + user messages.
		if len(req.Messages) != 2 {
			t.Errorf("want 2 messages, got %d", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]string{
				"role":    "assistant",
				"content": `{"result":"ok"}`,
			},
		})
	}))
	defer srv.Close()

	p := provider.NewOllama("llama3", srv.URL)
	got, _, err := p.Complete(context.Background(), "you are a bot", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"result":"ok"}` {
		t.Errorf("unexpected response: %q", got)
	}
}

func TestOllamaProvider_DefaultBaseURL(t *testing.T) {
	// NewOllama with empty baseURL should default to localhost:11434
	p := provider.NewOllama("llama3", "")
	// Can only verify it's non-nil; actual URL is internal state
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestOllamaProvider_NoSystemPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		// With no system prompt, only one user message should be sent
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
			t.Errorf("expected 1 user message, got %+v", req.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]string{"role": "assistant", "content": "ok"},
		})
	}))
	defer srv.Close()

	p := provider.NewOllama("llama3", srv.URL)
	_, _, err := p.Complete(context.Background(), "", "hello")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoPost_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json {{{"))
	}))
	defer srv.Close()

	p := provider.NewOllama("llama3", srv.URL)
	_, _, err := p.Complete(context.Background(), "", "hello")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestModelPrefix(t *testing.T) {
	cases := []struct {
		input    string
		wantPfx  string
		wantName string
	}{
		{"ollama/llama3", "ollama", "llama3"},
		{"claude-haiku-4-5", "", "claude-haiku-4-5"},
		{"ollama/gemma:7b", "ollama", "gemma:7b"},
	}
	for _, c := range cases {
		pfx, name := provider.ModelPrefix(c.input)
		if pfx != c.wantPfx || name != c.wantName {
			t.Errorf("ModelPrefix(%q) = (%q, %q), want (%q, %q)", c.input, pfx, name, c.wantPfx, c.wantName)
		}
	}
}
