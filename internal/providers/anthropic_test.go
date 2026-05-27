package providers

import (
	"context"
	"os"
	"testing"
)

func getTestAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("NURA_ANTHROPIC_API_KEY")
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		t.Skip("skipping: NURA_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY not set")
	}
	return key
}

func TestAnthropicProvider_Complete_Integration(t *testing.T) {
	key := getTestAPIKey(t)
	p := NewAnthropicProvider(key, "")

	resp, err := p.Complete(context.Background(), CompletionRequest{
		SystemPrompt: "Reply in exactly one short sentence.",
		UserPrompt:   "What is 2+2?",
		Temperature:  0.0,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
	if resp.Usage.InputTokens == 0 {
		t.Error("expected non-zero input tokens")
	}
	t.Logf("response: %s (in=%d, out=%d)", resp.Content, resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestAnthropicProvider_CompleteStream_Integration(t *testing.T) {
	key := getTestAPIKey(t)
	p := NewAnthropicProvider(key, "")

	ch, err := p.CompleteStream(context.Background(), CompletionRequest{
		UserPrompt:  "Say hello in Chinese in one word.",
		Temperature: 0.0,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var full string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		full += chunk.Delta
	}
	if full == "" {
		t.Error("expected non-empty streamed content")
	}
	t.Logf("streamed: %s", full)
}

func TestAnthropicProvider_Complete_WithTools_Integration(t *testing.T) {
	key := getTestAPIKey(t)
	p := NewAnthropicProvider(key, "")

	resp, err := p.Complete(context.Background(), CompletionRequest{
		UserPrompt:  "What is the weather in Tokyo?",
		Temperature: 0.0,
		Tools: []ToolDef{
			{
				Name:        "get_weather",
				Description: "Get weather for a city",
				Schema:      []byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("complete with tools: %v", err)
	}
	if len(resp.ToolCalls) == 0 {
		t.Log("model did not call tool (acceptable — depends on model judgment)")
		return
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "get_weather" {
		t.Errorf("tool name = %q, want 'get_weather'", tc.Name)
	}
	t.Logf("tool call: %s(%s)", tc.Name, string(tc.Input))
}

func TestNewProvider(t *testing.T) {
	// Mock provider.
	p, err := NewProvider("mock", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(*MockProvider); !ok {
		t.Error("expected MockProvider")
	}

	// Unknown provider.
	_, err = NewProvider("unknown", "", "")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}
