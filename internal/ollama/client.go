package ollama

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
)

// Request defines the payload sent to the Ollama API.
type Request struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Chunk mirrors the streamed response from Ollama.
type Chunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Client wraps the HTTP calls to the Ollama API.
type Client struct {
	http *http.Client
}

// NewClient builds a ready-to-use Ollama client.
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

// Generate sends a prompt to the model and returns the aggregated response.
func (c *Client) Generate(ctx context.Context, endpoint string, req Request) (string, error) {
	if !req.Stream {
		req.Stream = true
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	var out strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Bytes()
		var chunk Chunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		out.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}

	if err := sc.Err(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}
