package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/riskibarqy/go-commitgen/internal/llm"
)

type requestBody struct {
	Model       string                 `json:"model"`
	Messages    []message              `json:"messages"`
	Stream      bool                   `json:"stream"`
	Temperature float64                `json:"temperature,omitempty"`
	TopP        float64                `json:"top_p,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Text string `json:"text"`
	} `json:"choices"`
}

// Client wraps OpenAI-compatible chat completions calls.
type Client struct {
	http *http.Client
}

// NewClient builds a ready-to-use HTTP client.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			},
		},
	}
}

// Generate sends a prompt through an OpenAI-compatible chat completions endpoint.
func (c *Client) Generate(ctx context.Context, endpoint, apiKey string, req llm.Request) (string, error) {
	body := requestBody{
		Model: req.Model,
		Messages: []message{
			{Role: "user", Content: req.Prompt},
		},
		Stream:      true,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(endpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai-compatible API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			switch {
			case choice.Delta.Content != "":
				out.WriteString(choice.Delta.Content)
			case choice.Message.Content != "":
				out.WriteString(choice.Message.Content)
			case choice.Text != "":
				out.WriteString(choice.Text)
			}
		}
	}

	if err := sc.Err(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
