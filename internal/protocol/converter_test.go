package protocol

import (
	"bytes"
	"testing"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

func TestChatResponsesRoundTrip(t *testing.T) {
	chat := []byte(`{"model":"test","messages":[{"role":"system","content":"brief"},{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object"}}}]}`)
	responses, err := ConvertRequest(chat, models.FormatChatCompletions, models.FormatResponses)
	if err != nil || !bytes.Contains(responses, []byte(`"input"`)) || !bytes.Contains(responses, []byte(`"type":"function"`)) {
		t.Fatalf("Chat request conversion failed: %v, %s", err, responses)
	}
	back, err := ConvertRequest(responses, models.FormatResponses, models.FormatChatCompletions)
	if err != nil || !bytes.Contains(back, []byte(`"messages"`)) {
		t.Fatalf("Responses request conversion failed: %v, %s", err, back)
	}
}

func TestResponsesResponseConvertsToolCall(t *testing.T) {
	body, err := ConvertResponse([]byte(`{"id":"resp_1","status":"completed","model":"test","output":[{"type":"function_call","call_id":"call_1","name":"weather","arguments":"{}"}]}`), models.FormatResponses, models.FormatChatCompletions)
	if err != nil || !bytes.Contains(body, []byte(`"tool_calls"`)) || !bytes.Contains(body, []byte(`"finish_reason":"tool_calls"`)) {
		t.Fatalf("response conversion failed: %v, %s", err, body)
	}
}

func TestResponsesConversionRejectsServerState(t *testing.T) {
	_, err := ConvertRequest([]byte(`{"model":"test","previous_response_id":"resp_1","input":"hello"}`), models.FormatResponses, models.FormatChatCompletions)
	if err == nil {
		t.Fatal("expected previous_response_id to be rejected")
	}
}

func TestChatConversionRejectsStreamOptions(t *testing.T) {
	_, err := ConvertRequest([]byte(`{"model":"test","messages":[{"role":"user","content":"hello"}],"stream":true,"stream_options":{"include_usage":true}}`), models.FormatChatCompletions, models.FormatResponses)
	if err == nil {
		t.Fatal("expected stream_options to be rejected instead of silently dropping it")
	}
}
