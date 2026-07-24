// Package protocol converts the OpenAI Chat Completions and Responses wire
// formats. It deliberately accepts only the intersection the gateway can
// preserve; callers can then safely fall back instead of silently losing data.
package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

func CanConvert(from, to string) bool {
	return from == to || (from == models.FormatChatCompletions && to == models.FormatResponses) || (from == models.FormatResponses && to == models.FormatChatCompletions)
}

func ConvertRequest(body []byte, from, to string) ([]byte, error) {
	if from == to {
		return body, nil
	}
	if !CanConvert(from, to) {
		return nil, fmt.Errorf("unsupported format conversion %s -> %s", from, to)
	}
	var src map[string]any
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("invalid %s request: %w", from, err)
	}
	if from == models.FormatChatCompletions {
		return chatRequestToResponses(src)
	}
	return responsesRequestToChat(src)
}

func ConvertResponse(body []byte, from, to string) ([]byte, error) {
	if from == to {
		return body, nil
	}
	if !CanConvert(from, to) {
		return nil, fmt.Errorf("unsupported format conversion %s -> %s", from, to)
	}
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("data:")) {
		return convertStream(body, from, to)
	}
	var src map[string]any
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("invalid %s response: %w", from, err)
	}
	if from == models.FormatChatCompletions {
		return chatResponseToResponses(src)
	}
	return responsesResponseToChat(src)
}

func chatRequestToResponses(s map[string]any) ([]byte, error) {
	if err := reject(s, "functions", "function_call", "logit_bias", "response_format"); err != nil {
		return nil, err
	}
	if err := rejectUnknown(s, "model", "messages", "stream", "temperature", "top_p", "metadata", "max_tokens", "tools", "tool_choice"); err != nil {
		return nil, err
	}
	o := copyKeys(s, "model", "stream", "temperature", "top_p", "metadata")
	if value, ok := s["tool_choice"]; ok {
		choice, err := stringToolChoice(value)
		if err != nil {
			return nil, err
		}
		o["tool_choice"] = choice
	}
	if v, ok := s["max_tokens"]; ok {
		o["max_output_tokens"] = v
	}
	if v, ok := s["tools"]; ok {
		tools, err := chatToolsToResponses(v)
		if err != nil {
			return nil, err
		}
		o["tools"] = tools
	}
	msgs, ok := s["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("Chat messages must be an array")
	}
	input := make([]any, 0, len(msgs))
	for _, raw := range msgs {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Chat message")
		}
		role, _ := m["role"].(string)
		switch role {
		case "system", "developer", "user":
			content, err := chatContentToResponses(m["content"], "input_text")
			if err != nil {
				return nil, err
			}
			input = append(input, map[string]any{"type": "message", "role": role, "content": content})
		case "assistant":
			if c, exists := m["content"]; exists && c != nil {
				content, err := chatContentToResponses(c, "output_text")
				if err != nil {
					return nil, err
				}
				input = append(input, map[string]any{"type": "message", "role": "assistant", "content": content})
			}
			if calls, exists := m["tool_calls"]; exists {
				items, err := chatToolCallsToResponses(calls)
				if err != nil {
					return nil, err
				}
				input = append(input, items...)
			}
		case "tool":
			id, _ := m["tool_call_id"].(string)
			if id == "" {
				return nil, fmt.Errorf("Chat tool message has no tool_call_id")
			}
			content, ok := m["content"].(string)
			if !ok {
				return nil, fmt.Errorf("Chat tool result must be text")
			}
			input = append(input, map[string]any{"type": "function_call_output", "call_id": id, "output": content})
		default:
			return nil, fmt.Errorf("unsupported Chat message role %q", role)
		}
	}
	o["input"] = input
	return json.Marshal(o)
}

func responsesRequestToChat(s map[string]any) ([]byte, error) {
	if err := reject(s, "previous_response_id", "background", "reasoning", "include", "truncation", "service_tier"); err != nil {
		return nil, err
	}
	if err := rejectUnknown(s, "model", "input", "instructions", "stream", "temperature", "top_p", "metadata", "max_output_tokens", "tools", "tool_choice"); err != nil {
		return nil, err
	}
	o := copyKeys(s, "model", "stream", "temperature", "top_p", "metadata")
	if value, ok := s["tool_choice"]; ok {
		choice, err := stringToolChoice(value)
		if err != nil {
			return nil, err
		}
		o["tool_choice"] = choice
	}
	if v, ok := s["max_output_tokens"]; ok {
		o["max_tokens"] = v
	}
	if v, ok := s["tools"]; ok {
		tools, err := responsesToolsToChat(v)
		if err != nil {
			return nil, err
		}
		o["tools"] = tools
	}
	msgs := []any{}
	if instructions, ok := s["instructions"].(string); ok && instructions != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": instructions})
	}
	switch in := s["input"].(type) {
	case string:
		msgs = append(msgs, map[string]any{"role": "user", "content": in})
	case []any:
		for _, raw := range in {
			m, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unsupported Responses input item")
			}
			typ, _ := m["type"].(string)
			switch typ {
			case "", "message":
				role, _ := m["role"].(string)
				if role == "" {
					role = "user"
				}
				c, err := responsesContentToChat(m["content"])
				if err != nil {
					return nil, err
				}
				msgs = append(msgs, map[string]any{"role": role, "content": c})
			case "function_call_output":
				id, _ := m["call_id"].(string)
				out, _ := m["output"].(string)
				if id == "" {
					return nil, fmt.Errorf("function_call_output has no call_id")
				}
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": id, "content": out})
			case "function_call":
				id, _ := m["call_id"].(string)
				name, _ := m["name"].(string)
				args, _ := m["arguments"].(string)
				if id == "" || name == "" {
					return nil, fmt.Errorf("invalid function_call")
				}
				msgs = append(msgs, map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": args}}}})
			default:
				return nil, fmt.Errorf("unsupported Responses input type %q", typ)
			}
		}
	default:
		return nil, fmt.Errorf("Responses input must be a string or array")
	}
	o["messages"] = msgs
	return json.Marshal(o)
}

func chatResponseToResponses(s map[string]any) ([]byte, error) {
	choices, ok := s["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("Chat response has no choices")
	}
	c, ok := choices[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid Chat choice")
	}
	msg, ok := c["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Chat response has no assistant message")
	}
	items := []any{}
	content := []any{}
	if text, ok := msg["content"].(string); ok && text != "" {
		content = append(content, map[string]any{"type": "output_text", "text": text})
	}
	if reasoning, ok := msg["reasoning_content"].(string); ok && reasoning != "" {
		items = append(items, map[string]any{"type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": reasoning}}})
	}
	if len(content) > 0 {
		items = append(items, map[string]any{"id": "msg_" + str(s["id"]), "type": "message", "role": "assistant", "content": content})
	}
	if calls, exists := msg["tool_calls"]; exists {
		x, err := chatToolCallsToResponses(calls)
		if err != nil {
			return nil, err
		}
		items = append(items, x...)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("Chat response has no convertible output")
	}
	o := map[string]any{"id": "resp_" + str(s["id"]), "object": "response", "status": "completed", "model": s["model"], "output": items}
	if u, ok := s["usage"].(map[string]any); ok {
		o["usage"] = map[string]any{"input_tokens": u["prompt_tokens"], "output_tokens": u["completion_tokens"], "total_tokens": u["total_tokens"]}
	}
	return json.Marshal(o)
}

func responsesResponseToChat(s map[string]any) ([]byte, error) {
	if status, _ := s["status"].(string); status != "" && status != "completed" {
		return nil, fmt.Errorf("Responses response status is %q", status)
	}
	items, ok := s["output"].([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("Responses response has no output")
	}
	text := strings.Builder{}
	reasoning := strings.Builder{}
	calls := []any{}
	for _, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Responses output item")
		}
		switch it["type"] {
		case "message":
			parts, ok := it["content"].([]any)
			if !ok {
				return nil, fmt.Errorf("invalid Responses message")
			}
			for _, p := range parts {
				pm, ok := p.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid Responses content")
				}
				if pm["type"] != "output_text" {
					return nil, fmt.Errorf("unsupported Responses output content %q", pm["type"])
				}
				t, _ := pm["text"].(string)
				text.WriteString(t)
			}
		case "function_call":
			id, _ := it["call_id"].(string)
			name, _ := it["name"].(string)
			args, _ := it["arguments"].(string)
			if id == "" || name == "" {
				return nil, fmt.Errorf("invalid Responses function_call")
			}
			calls = append(calls, map[string]any{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": args}})
		case "reasoning":
			if _, encrypted := it["encrypted_content"]; encrypted {
				return nil, fmt.Errorf("encrypted reasoning cannot be converted")
			}
			summary, ok := it["summary"].([]any)
			if !ok {
				return nil, fmt.Errorf("unmappable Responses reasoning item")
			}
			for _, raw := range summary {
				part, ok := raw.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid Responses reasoning item")
				}
				value, _ := part["text"].(string)
				reasoning.WriteString(value)
			}
		default:
			return nil, fmt.Errorf("unsupported Responses output item %q", it["type"])
		}
	}
	message := map[string]any{"role": "assistant", "content": text.String()}
	if reasoning.Len() > 0 {
		message["reasoning_content"] = reasoning.String()
	}
	if len(calls) > 0 {
		message["tool_calls"] = calls
	}
	finish := "stop"
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	o := map[string]any{"id": "chatcmpl-" + str(s["id"]), "object": "chat.completion", "model": s["model"], "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}}
	if u, ok := s["usage"].(map[string]any); ok {
		o["usage"] = map[string]any{"prompt_tokens": u["input_tokens"], "completion_tokens": u["output_tokens"], "total_tokens": u["total_tokens"]}
	}
	return json.Marshal(o)
}

func copyKeys(s map[string]any, keys ...string) map[string]any {
	o := map[string]any{}
	for _, k := range keys {
		if v, ok := s[k]; ok {
			o[k] = v
		}
	}
	return o
}
func str(v any) string {
	if x, ok := v.(string); ok {
		return x
	}
	return "gateway"
}
func reject(s map[string]any, keys ...string) error {
	for _, k := range keys {
		if _, ok := s[k]; ok {
			return fmt.Errorf("unsupported field %q", k)
		}
	}
	return nil
}

func rejectUnknown(s map[string]any, allowed ...string) error {
	known := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		known[key] = struct{}{}
	}
	for key := range s {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("unsupported field %q", key)
		}
	}
	return nil
}

func stringToolChoice(value any) (string, error) {
	choice, ok := value.(string)
	if !ok || (choice != "auto" && choice != "none" && choice != "required") {
		return "", fmt.Errorf("unsupported tool_choice")
	}
	return choice, nil
}

func chatContentToResponses(v any, kind string) ([]any, error) {
	if text, ok := v.(string); ok {
		return []any{map[string]any{"type": kind, "text": text}}, nil
	}
	parts, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("unsupported Chat message content")
	}
	out := []any{}
	for _, raw := range parts {
		p, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Chat content part")
		}
		switch p["type"] {
		case "text":
			out = append(out, map[string]any{"type": kind, "text": p["text"]})
		case "image_url":
			image, ok := p["image_url"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid image_url")
			}
			out = append(out, map[string]any{"type": "input_image", "image_url": image["url"]})
		default:
			return nil, fmt.Errorf("unsupported Chat content type %q", p["type"])
		}
	}
	return out, nil
}
func responsesContentToChat(v any) (any, error) {
	if text, ok := v.(string); ok {
		return text, nil
	}
	parts, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("unsupported Responses message content")
	}
	out := []any{}
	for _, raw := range parts {
		p, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Responses content part")
		}
		switch p["type"] {
		case "input_text", "output_text":
			out = append(out, map[string]any{"type": "text", "text": p["text"]})
		case "input_image":
			out = append(out, map[string]any{"type": "image_url", "image_url": map[string]any{"url": p["image_url"]}})
		default:
			return nil, fmt.Errorf("unsupported Responses content type %q", p["type"])
		}
	}
	return out, nil
}
func chatToolsToResponses(v any) ([]any, error) {
	tools, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("Chat tools must be an array")
	}
	out := []any{}
	for _, raw := range tools {
		t, ok := raw.(map[string]any)
		if !ok || t["type"] != "function" {
			return nil, fmt.Errorf("only function tools can be converted")
		}
		f, ok := t["function"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Chat function tool")
		}
		o := copyKeys(f, "name", "description", "parameters")
		o["type"] = "function"
		out = append(out, o)
	}
	return out, nil
}
func responsesToolsToChat(v any) ([]any, error) {
	tools, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("Responses tools must be an array")
	}
	out := []any{}
	for _, raw := range tools {
		t, ok := raw.(map[string]any)
		if !ok || t["type"] != "function" {
			return nil, fmt.Errorf("only function tools can be converted")
		}
		f := copyKeys(t, "name", "description", "parameters")
		delete(f, "type")
		out = append(out, map[string]any{"type": "function", "function": f})
	}
	return out, nil
}
func chatToolCallsToResponses(v any) ([]any, error) {
	calls, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("Chat tool_calls must be an array")
	}
	out := []any{}
	for _, raw := range calls {
		c, ok := raw.(map[string]any)
		if !ok || c["type"] != "function" {
			return nil, fmt.Errorf("only function tool calls can be converted")
		}
		f, ok := c["function"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Chat tool call")
		}
		id, _ := c["id"].(string)
		name, _ := f["name"].(string)
		args, _ := f["arguments"].(string)
		if id == "" || name == "" {
			return nil, fmt.Errorf("invalid Chat tool call")
		}
		out = append(out, map[string]any{"type": "function_call", "call_id": id, "name": name, "arguments": args})
	}
	return out, nil
}

// convertStream performs event-level conversion. It supports text, reasoning,
// tool-call argument deltas, completion and usage; unfamiliar events fail
// instead of leaking a different protocol to the client.
func convertStream(body []byte, from, to string) ([]byte, error) {
	var out bytes.Buffer
	for _, block := range bytes.Split(body, []byte("\n\n")) {
		line := bytes.TrimSpace(block)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			return nil, fmt.Errorf("invalid SSE event")
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if bytes.Equal(data, []byte("[DONE]")) {
			if to == models.FormatChatCompletions {
				out.WriteString("data: [DONE]\n\n")
			}
			continue
		}
		var e map[string]any
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("invalid SSE JSON: %w", err)
		}
		converted, err := convertEvent(e, from, to)
		if err != nil {
			return nil, err
		}
		for _, v := range converted {
			b, _ := json.Marshal(v)
			out.WriteString("data: ")
			out.Write(b)
			out.WriteString("\n\n")
		}
	}
	return out.Bytes(), nil
}
func convertEvent(e map[string]any, from, to string) ([]map[string]any, error) {
	if from == models.FormatChatCompletions {
		choices, _ := e["choices"].([]any)
		if len(choices) == 0 {
			return nil, fmt.Errorf("unsupported Chat stream event")
		}
		c, _ := choices[0].(map[string]any)
		d, _ := c["delta"].(map[string]any)
		if d == nil {
			return nil, fmt.Errorf("invalid Chat stream delta")
		}
		events := []map[string]any{}
		if x, ok := d["content"].(string); ok {
			events = append(events, map[string]any{"type": "response.output_text.delta", "delta": x})
		}
		if x, ok := d["reasoning_content"].(string); ok {
			events = append(events, map[string]any{"type": "response.reasoning.delta", "delta": x})
		}
		if calls, ok := d["tool_calls"].([]any); ok {
			for _, raw := range calls {
				tc, _ := raw.(map[string]any)
				idx := tc["index"]
				f, _ := tc["function"].(map[string]any)
				if name, ok := f["name"].(string); ok {
					events = append(events, map[string]any{"type": "response.output_item.added", "output_index": idx, "item": map[string]any{"type": "function_call", "call_id": tc["id"], "name": name, "arguments": ""}})
				}
				if a, ok := f["arguments"].(string); ok {
					events = append(events, map[string]any{"type": "response.function_call_arguments.delta", "output_index": idx, "delta": a})
				}
			}
		}
		if finish, _ := c["finish_reason"].(string); finish != "" {
			events = append(events, map[string]any{"type": "response.completed"})
		}
		if u, ok := e["usage"].(map[string]any); ok {
			events = append(events, map[string]any{"type": "response.completed", "response": map[string]any{"usage": map[string]any{"input_tokens": u["prompt_tokens"], "output_tokens": u["completion_tokens"], "total_tokens": u["total_tokens"]}}})
		}
		if len(events) == 0 {
			return nil, fmt.Errorf("unsupported Chat stream delta")
		}
		return events, nil
	}
	typ, _ := e["type"].(string)
	switch typ {
	case "response.output_text.delta":
		return []map[string]any{{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": e["delta"]}}}}}, nil
	case "response.reasoning.delta":
		return []map[string]any{{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": e["delta"]}}}}}, nil
	case "response.output_item.added":
		item, ok := e["item"].(map[string]any)
		if !ok || item["type"] != "function_call" {
			return nil, fmt.Errorf("unsupported Responses output item event")
		}
		return []map[string]any{{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": e["output_index"], "id": item["call_id"], "type": "function", "function": map[string]any{"name": item["name"]}}}}}}}}, nil
	case "response.function_call_arguments.delta":
		return []map[string]any{{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": e["output_index"], "type": "function", "function": map[string]any{"arguments": e["delta"]}}}}}}}}, nil
	case "response.completed":
		return []map[string]any{{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}}}, nil
	default:
		return nil, fmt.Errorf("unsupported Responses stream event %q", typ)
	}
}
