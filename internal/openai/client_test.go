package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/riskibarqy/go-commitgen/internal/llm"
)

func TestClientGenerateStreamsContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/v1/chat/completions"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer secret"; got != want {
			t.Fatalf("authorization = %q, want %q", got, want)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"hello "}}]}`)
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"world"}}]}`)
		_, _ = fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	got, err := client.Generate(context.Background(), server.URL+"/v1", "secret", llm.Request{
		Model:       "9router-codex",
		Prompt:      "hi",
		Temperature: 0.2,
		TopP:        0.9,
		MaxTokens:   120,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "hello world" {
		t.Fatalf("Generate() = %q, want %q", got, "hello world")
	}
}

func TestClientGenerateReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad upstream", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	_, err := client.Generate(context.Background(), server.URL+"/v1", "", llm.Request{
		Model:  "9router-codex",
		Prompt: "hi",
	})
	if err == nil {
		t.Fatal("Generate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "openai-compatible API error 502") {
		t.Fatalf("Generate() error = %q", err)
	}
}
