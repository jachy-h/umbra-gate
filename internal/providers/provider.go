package providers

import (
	"context"
	"errors"
)

// OpenAIReq carries the raw provider-style payload plus the small set of fields
// needed for model fallback and legacy adapters. Raw is authoritative so the
// official SDK clients preserve unknown request fields.
type OpenAIReq struct {
	Model     string          `json:"model"`
	Messages  []OpenAIMessage `json:"messages"`
	Stream    bool            `json:"stream,omitempty"`
	MaxTokens int             `json:"max_tokens,omitempty"`
	Extra     map[string]any  `json:"-"`
	Raw       []byte          `json:"-"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Result carries the provider-native response body, upstream status, and the
// exact HTTP metadata needed by request diagnostics.
type Result struct {
	Body            []byte
	StatusCode      int
	Err             error
	RequestURL      string
	RequestHeaders  map[string]string
	RequestBody     []byte
	ResponseHeaders map[string]string
}

// Adapter invokes one provider style. OpenAI and Anthropic adapters keep their
// native wire contracts; only explicitly asymmetric endpoints are adapted by
// the forwarding layer.
type Adapter interface {
	Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride, protocol string) Result
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
