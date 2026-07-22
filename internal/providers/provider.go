package providers

import (
	"context"
	"errors"
)

// OpenAIReq is a minimal OpenAI Chat Completions request representation used
// internally for conversion. We keep most fields as raw JSON to preserve
// unknown fields while still allowing conversions for Anthropic/Gemini.
type OpenAIReq struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
	Extra    map[string]any  `json:"-"`
	Raw      []byte          `json:"-"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Result carries a normalized OpenAI-compatible response body, the upstream
// status code, and a provider-visible error (if any).
type Result struct {
	Body       []byte
	StatusCode int
	Err        error
}

// Adapter converts an OpenAI Chat Completions request to a provider-native
// request and invokes the upstream, returning an OpenAI-compatible response.
type Adapter interface {
	Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride string) Result
	Type() string
}

type Provider struct {
	ID      string
	Name    string
	Type    string
	BaseURL string
	APIKey  string
	Models  []string
	Extra   map[string]any
}

var ErrNoAdapter = errors.New("no adapter registered for provider type")
