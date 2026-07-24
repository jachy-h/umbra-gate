package models

import "time"

const (
	ProtocolOpenAI    = "openai"
	ProtocolAnthropic = "anthropic"

	FormatChatCompletions = "chat_completions"
	FormatResponses       = "responses"
	FormatMessages        = "messages"
)

type ProviderEndpoint struct {
	Protocol       string `json:"protocol"`
	RequestFormat  string `json:"request_format"`
	ResponseFormat string `json:"response_format"`
	BaseURL        string `json:"base_url"`
}

type Provider struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Type      string             `json:"type"`     // openai | anthropic | gemini | deepseek | qwen | custom
	BaseURL   string             `json:"base_url"` // legacy primary URL; endpoints is authoritative
	Endpoints []ProviderEndpoint `json:"endpoints"`
	APIKey    string             `json:"api_key,omitempty"`
	Models    []string           `json:"models"`
	Extra     Map                `json:"extra,omitempty"`
	Enabled   bool               `json:"enabled"`
	Builtin   bool               `json:"builtin"`
	HasAPIKey bool               `json:"has_api_key"`
	CreatedAt time.Time          `json:"created_at"`
}

type Map map[string]any

type ChainEntry struct {
	ProviderID      string    `json:"provider_id"`
	Protocol        string    `json:"protocol"`
	RetryCount      int       `json:"retry_count"`       // extra retries on same provider before fallback
	FallbackModel   string    `json:"fallback_model"`    // optional model override when falling back
	ApiKey          string    `json:"api_key,omitempty"` // override provider's global api key
	Rules           Rules     `json:"rules,omitempty"`   // when to fallback
	ValidationOK    *bool     `json:"validation_ok,omitempty"`
	ValidationError string    `json:"validation_error,omitempty"`
	ValidatedAt     time.Time `json:"validated_at,omitempty"`
	// SupportedFormats is discovered by validation probes, rather than inferred
	// from a provider type or a manually declared endpoint shape.
	SupportedFormats []string `json:"supported_formats,omitempty"`
}

type Rules struct {
	OnStatusCodes []int    `json:"on_status_codes"` // e.g. [429,500,503]
	OnErrors      []string `json:"on_errors"`       // substring match on error message
	OnTimeout     bool     `json:"on_timeout"`
}

type ProxyLink struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"` // proxy url token
	Protocol string `json:"protocol"`
	// SupportedFormats is the intersection of every chain entry's discovered
	// formats. It tells callers which operation URLs this Link can serve.
	SupportedFormats []string     `json:"supported_formats,omitempty"`
	Attributes       Map          `json:"attributes"` // for stats grouping
	Chain            []ChainEntry `json:"chain"`
	Enabled          bool         `json:"enabled"`
	CreatedAt        time.Time    `json:"created_at"`
}

type RequestLog struct {
	ID              string    `json:"id"`
	LinkID          string    `json:"link_id"`
	Path            string    `json:"path"`
	ProviderID      string    `json:"provider_id"`
	ProviderName    string    `json:"provider_name"`
	Model           string    `json:"model"`
	StatusCode      int       `json:"status_code"`
	LatencyMS       int64     `json:"latency_ms"`
	Success         bool      `json:"success"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	RequestURL      string    `json:"request_url,omitempty"`
	RequestHeaders  Map       `json:"request_headers,omitempty"`
	RequestBody     string    `json:"request_body,omitempty"`
	UpstreamURL     string    `json:"upstream_url,omitempty"`
	UpstreamHeaders Map       `json:"upstream_headers,omitempty"`
	UpstreamBody    string    `json:"upstream_body,omitempty"`
	ResponseHeaders Map       `json:"response_headers,omitempty"`
	ResponseBody    string    `json:"response_body,omitempty"`
	Attributes      Map       `json:"attributes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Stats struct {
	LinkID     string `json:"link_id"`
	ProviderID string `json:"provider_id"`
	Period     string `json:"period"` // hour bucket key e.g. 2026-07-21T18
	Total      int64  `json:"total"`
	Success    int64  `json:"success"`
	Failure    int64  `json:"failure"`
	TotalLatMS int64  `json:"total_latency_ms"`
}
