package providers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// anthropicToOpenAI converts a /v1/messages response to OpenAI Chat
// Completions response format (non-streaming, simplified).
func anthropicToOpenAI(body []byte) ([]byte, error) {
	var src map[string]any
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, err
	}
	var textParts []string
	if content, ok := src["content"].([]any); ok {
		for _, c := range content {
			if m, ok := c.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					textParts = append(textParts, t)
				}
			}
		}
	}
	usage := map[string]any{}
	if u, ok := src["usage"].(map[string]any); ok {
		usage["prompt_tokens"] = u["input_tokens"]
		usage["completion_tokens"] = u["output_tokens"]
		usage["total_tokens"] = toInt(u["input_tokens"]) + toInt(u["output_tokens"])
	}
	out := map[string]any{
		"id":     src["id"],
		"object": "chat.completion",
		"model":  src["model"],
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": strings.Join(textParts, ""),
			},
			"finish_reason": "stop",
		}},
		"usage": usage,
	}
	return json.Marshal(out)
}

// geminiToOpenAI converts a generateContent response to OpenAI Chat
// Completions response format (non-streaming, simplified).
func geminiToOpenAI(body []byte) ([]byte, error) {
	var src map[string]any
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, err
	}
	var texts []string
	if candidates, ok := src["candidates"].([]any); ok && len(candidates) > 0 {
		if c, ok := candidates[0].(map[string]any); ok {
			if content, ok := c["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok {
					for _, p := range parts {
						if m, ok := p.(map[string]any); ok {
							if t, ok := m["text"].(string); ok {
								texts = append(texts, t)
							}
						}
					}
				}
			}
		}
	}
	usage := map[string]any{}
	if u, ok := src["usageMetadata"].(map[string]any); ok {
		usage["prompt_tokens"] = u["promptTokenCount"]
		usage["completion_tokens"] = u["candidatesTokenCount"]
		usage["total_tokens"] = u["totalTokenCount"]
	}
	out := map[string]any{
		"id":     "chatcmpl-gemini",
		"object": "chat.completion",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": strings.Join(texts, ""),
			},
			"finish_reason": "stop",
		}},
		"usage": usage,
	}
	return json.Marshal(out)
}

func toInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	default:
		var p int64
		fmt.Sscanf(fmt.Sprint(v), "%d", &p)
		return p
	}
}
