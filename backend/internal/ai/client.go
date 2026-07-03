package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the LLM transport used by the Analyzer; fakes implement it in tests.
type Client interface {
	// ChatJSON sends a system+user message pair and returns the raw
	// assistant text, which is expected to be JSON.
	ChatJSON(ctx context.Context, system, user string) (string, error)
}

// ClientConfig configures the OpenAI-compatible client. BaseURL may point at
// OpenAI, Azure, OpenRouter, Ollama, or any compatible server.
type ClientConfig struct {
	APIKey  string
	BaseURL string // e.g. https://api.openai.com/v1
	Model   string
	Timeout time.Duration
}

// OpenAIClient talks to any OpenAI-compatible /chat/completions endpoint.
type OpenAIClient struct {
	cfg  ClientConfig
	http *http.Client
}

// NewOpenAIClient builds the production client.
func NewOpenAIClient(cfg ClientConfig) *OpenAIClient {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second
	}
	return &OpenAIClient{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat *respFormat   `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type respFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ChatJSON performs one chat completion demanding a JSON object response.
// One retry on transient failures (5xx, 429, transport errors).
func (c *OpenAIClient) ChatJSON(ctx context.Context, system, user string) (string, error) {
	payload := chatRequest{
		Model: c.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    0.1,
		ResponseFormat: &respFormat{Type: "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		out, retryable, err := c.doRequest(ctx, body)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retryable {
			break
		}
	}
	return "", lastErr
}

func (c *OpenAIClient) doRequest(ctx context.Context, body []byte) (out string, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", true, fmt.Errorf("read ai response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", true, fmt.Errorf("ai http %d: %s", resp.StatusCode, snippet(data))
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("ai http %d: %s", resp.StatusCode, snippet(data))
	}

	var parsed chatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", false, fmt.Errorf("decode ai response: %w", err)
	}
	if parsed.Error != nil {
		return "", false, fmt.Errorf("ai error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", false, fmt.Errorf("ai response has no choices")
	}
	return parsed.Choices[0].Message.Content, false, nil
}

func snippet(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}
