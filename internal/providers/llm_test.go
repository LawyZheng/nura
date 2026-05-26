package providers

import (
	"context"
	"testing"
)

func TestMockProvider_Complete(t *testing.T) {
	m := &MockProvider{}
	resp, err := m.Complete(context.Background(), CompletionRequest{
		UserPrompt: "test prompt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
	if resp.Usage.InputTokens == 0 {
		t.Error("expected non-zero input tokens")
	}
}

func TestMockProvider_CompleteWithCustomFunc(t *testing.T) {
	m := &MockProvider{
		CompleteFunc: func(_ context.Context, req CompletionRequest) (*CompletionResponse, error) {
			return &CompletionResponse{
				Content: "custom: " + req.UserPrompt,
			}, nil
		},
	}
	resp, err := m.Complete(context.Background(), CompletionRequest{
		UserPrompt: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "custom: hello" {
		t.Errorf("content = %q, want 'custom: hello'", resp.Content)
	}
}

func TestMockProvider_CompleteStream(t *testing.T) {
	m := &MockProvider{}
	ch, err := m.CompleteStream(context.Background(), CompletionRequest{
		UserPrompt: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if !chunks[0].Done {
		t.Error("expected final chunk to be done")
	}
}

func TestMockProvider_CompleteWithVision(t *testing.T) {
	m := &MockProvider{}
	resp, err := m.CompleteWithVision(context.Background(), VisionRequest{
		CompletionRequest: CompletionRequest{UserPrompt: "describe this image"},
		ImageData:         []byte("fake image data"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestMockProvider_ImplementsInterface(t *testing.T) {
	var _ LLMProvider = &MockProvider{}
}
