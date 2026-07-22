package providers

import (
	"sync"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

var (
	regMu    sync.RWMutex
	regOnce  sync.Once
	registry = map[string]Adapter{}
)

// Register adds (or replaces) an adapter for a provider type.
func Register(a Adapter) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[a.Type()] = a
}

func AdapterFor(typeName string) (Adapter, bool) {
	regOnce.Do(initBuiltins)
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := registry[typeName]
	return a, ok
}

func initBuiltins() {
	regMu.Lock()
	defer regMu.Unlock()
	registry["openai"] = newOpenAI("openai", "/v1/chat/completions")
	registry["deepseek"] = newOpenAI("deepseek", "/v1/chat/completions")
	registry["qwen"] = newOpenAI("qwen", "/v1/chat/completions")
	registry["custom"] = newOpenAI("custom", "/v1/chat/completions")
	registry["opencode"] = newOpenAI("opencode", "")
	registry["anthropic"] = AnthropicAdapter{}
	registry["gemini"] = GeminiAdapter{}
}

// FromModel converts a stored provider model into the runtime Provider.
func FromModel(m models.Provider) Provider {
	return Provider{
		ID: m.ID, Name: m.Name, Type: m.Type,
		BaseURL: m.BaseURL, APIKey: m.APIKey,
		Models: m.Models, Extra: m.Extra,
	}
}

// RegisteredTypes returns the available provider type names.
func RegisteredTypes() []string {
	regOnce.Do(initBuiltins)
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
