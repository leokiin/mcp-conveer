package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leokiin/mcp-conveer/internal/provider"
)

func TestDeepSeekProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}

		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "deepseek-v4-flash" {
			t.Errorf("want model deepseek-v4-flash, got %s", req.Model)
		}
		if len(req.Messages) < 1 {
			t.Errorf("want at least 1 message, got %d", len(req.Messages))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "hello from deepseek"}},
			},
		})
	}))
	defer srv.Close()

	p := provider.NewDeepSeekWithURL("deepseek-v4-flash", "test-key", srv.URL)
	got, _, err := p.Complete(context.Background(), "you are a bot", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello from deepseek" {
		t.Errorf("unexpected response: %q", got)
	}
}

func TestDeepSeekProvider_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{}})
	}))
	defer srv.Close()

	p := provider.NewDeepSeekWithURL("deepseek-v4-flash", "test-key", srv.URL)
	_, _, err := p.Complete(context.Background(), "", "hello")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDoPost_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := provider.NewOllama("llama3", srv.URL)
	_, _, err := p.Complete(context.Background(), "", "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("want status 429 in error, got: %v", err)
	}
}
