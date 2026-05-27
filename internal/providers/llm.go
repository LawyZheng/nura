package providers

import (
	"context"
	"encoding/json"
	"fmt"
)

// CompletionRequest represents a text completion request to an LLM.
type CompletionRequest struct {
	SystemPrompt string          `json:"system_prompt,omitempty"`
	UserPrompt   string          `json:"user_prompt"`
	Temperature  float64         `json:"temperature,omitempty"` // default 0.1 for medical accuracy
	Tools        []ToolDef       `json:"tools,omitempty"`
	Schema       json.RawMessage `json:"schema,omitempty"` // structured output schema
}

// ToolDef describes a tool for the LLM to potentially call.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// CompletionResponse is the result of an LLM completion.
type CompletionResponse struct {
	Content    string          `json:"content"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	Usage      Usage           `json:"usage"`
	StopReason string          `json:"stop_reason,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamChunk is one piece of a streaming completion response.
type StreamChunk struct {
	Delta string `json:"delta,omitempty"`
	Done  bool   `json:"done"`
	Error error  `json:"-"`
}

// VisionRequest extends CompletionRequest with image data.
type VisionRequest struct {
	CompletionRequest
	ImageData []byte `json:"image_data,omitempty"`
	ImageURL  string `json:"image_url,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
}

// LLMProvider abstracts LLM access so implementations can be swapped.
type LLMProvider interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
	CompleteWithVision(ctx context.Context, req VisionRequest) (*CompletionResponse, error)
}

// NewProvider creates an LLMProvider by name. Supported: "mock", "anthropic".
func NewProvider(name, apiKey, model string) (LLMProvider, error) {
	switch name {
	case "mock", "":
		return &MockProvider{}, nil
	case "anthropic":
		return NewAnthropicProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (supported: mock, anthropic)", name)
	}
}

// MockProvider is a stub LLM provider for testing without real API keys.
type MockProvider struct {
	CompleteFunc           func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	CompleteStreamFunc     func(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
	CompleteWithVisionFunc func(ctx context.Context, req VisionRequest) (*CompletionResponse, error)
}

func (m *MockProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return &CompletionResponse{
		Content:    "Mock response for: " + req.UserPrompt,
		StopReason: "end_turn",
		Usage:      Usage{InputTokens: 10, OutputTokens: 20},
	}, nil
}

func (m *MockProvider) CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	if m.CompleteStreamFunc != nil {
		return m.CompleteStreamFunc(ctx, req)
	}
	ch := make(chan StreamChunk, 1)
	go func() {
		defer close(ch)
		ch <- StreamChunk{Delta: "Mock streaming response", Done: true}
	}()
	return ch, nil
}

func (m *MockProvider) CompleteWithVision(ctx context.Context, req VisionRequest) (*CompletionResponse, error) {
	if m.CompleteWithVisionFunc != nil {
		return m.CompleteWithVisionFunc(ctx, req)
	}
	return &CompletionResponse{
		Content:    "Mock vision response",
		StopReason: "end_turn",
		Usage:      Usage{InputTokens: 100, OutputTokens: 50},
	}, nil
}
