package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"find-restaurants/internal/apperrors"
	"find-restaurants/internal/config"
)

type ChatJSONRequest struct {
	Operation string
	System    string
	User      string
	MaxTokens int
}

type Client struct {
	config config.LLMConfig
	http   *http.Client
}

func NewClient(cfg config.LLMConfig) *Client {
	return &Client{
		config: cfg,
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) ChatJSON(ctx context.Context, req ChatJSONRequest, out any) error {
	if c.config.APIKey == "" {
		return apperrors.New(500, "missing_llm_api_key", "Missing LLM API key. Set OPENROUTER_API_KEY, or set LLM_PROVIDER=openai with OPENAI_API_KEY.")
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1200
	}

	body := chatCompletionRequest{
		Model: c.config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
		Temperature:    0.1,
		MaxTokens:      maxTokens,
		ResponseFormat: map[string]string{"type": "json_object"},
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.Endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.Provider == "openrouter" {
		httpReq.Header.Set("HTTP-Referer", c.config.SiteURL)
		httpReq.Header.Set("X-Title", c.config.AppName)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return apperrors.New(502, "llm_request_failed", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return serviceError(resp, "llm_request_failed")
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return err
	}

	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return apperrors.New(502, "empty_llm_response", req.Operation+" returned an empty response")
	}

	content := cleanJSONContent(decoded.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(content), out); err != nil {
		return fmt.Errorf("%s returned invalid JSON: %w", req.Operation, err)
	}

	return nil
}

type chatCompletionRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func serviceError(resp *http.Response, code string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 700))
	status := 500
	if resp.StatusCode >= 500 {
		status = 502
	}
	return apperrors.New(status, code, fmt.Sprintf("LLM request failed with %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
}

func cleanJSONContent(content string) string {
	value := strings.TrimSpace(content)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	return strings.TrimSpace(value)
}
