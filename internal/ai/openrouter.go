package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// OpenRouter calls the chat completions API with one capped key (PRD
// Constraints). Requests ask for usage so the cost is recorded, and route
// only to providers that do not keep or train on prompts (T-31).
type OpenRouter struct {
	key    string
	model  string
	url    string
	client *http.Client
}

// NewOpenRouter builds the client; the key is read from the environment by config.
func NewOpenRouter(key, model string) *OpenRouter {
	return &OpenRouter{key: key, model: model, url: "https://openrouter.ai/api/v1/chat/completions", client: &http.Client{}}
}

// Name identifies the provider in logs.
func (o *OpenRouter) Name() string { return "openrouter" }

type orMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

// Complete sends one request and maps the outcome to the gateway's errors.
func (o *OpenRouter) Complete(ctx context.Context, req Request) (Response, error) {
	content := []any{map[string]string{"type": "text", "text": req.Prompt}}
	if len(req.Image) > 0 {
		url := "data:" + req.ImageType + ";base64," + base64.StdEncoding.EncodeToString(req.Image)
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": url}})
	}
	body, err := json.Marshal(map[string]any{
		"model":       o.model,
		"messages":    []orMessage{{Role: "user", Content: content}},
		"max_tokens":  800,
		"temperature": 0.3,
		"usage":       map[string]bool{"include": true},
		"provider":    map[string]string{"data_collection": "deny"},
	})
	if err != nil {
		return Response{}, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.key)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := o.client.Do(httpReq)
	if err != nil {
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			return Response{}, fmt.Errorf("%w: timed out: %w", ErrUncertain, err)
		}
		return Response{}, fmt.Errorf("%w: %w", ErrUncertain, err)
	}
	defer func() { _ = res.Body.Close() }() // nothing useful to do if close fails
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Response{}, fmt.Errorf("%w: read body: %w", ErrUncertain, err)
	}
	if res.StatusCode == http.StatusTooManyRequests {
		wait := 30 * time.Second
		if s, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && s > 0 {
			wait = time.Duration(s) * time.Second
		}
		return Response{}, &RateLimitedError{RetryAfter: wait}
	}
	if res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("openrouter returned %d", res.StatusCode)
	}
	var out struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("openrouter answer could not be read")
	}
	return Response{
		Text:         out.Choices[0].Message.Content,
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
		CostMicroUSD: int64(out.Usage.Cost * 1_000_000),
		GenerationID: out.ID,
		Model:        out.Model,
	}, nil
}
