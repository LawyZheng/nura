package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

// AnthropicProvider implements LLMProvider using the Anthropic Claude API.
type AnthropicProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

// NewAnthropicProvider creates a provider backed by the Claude API.
// apiKey is read from config/env. model defaults to claude-sonnet-4-20250514.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	m := anthropic.Model(model)
	if model == "" {
		m = anthropic.ModelClaudeSonnet4_20250514
	}
	return &AnthropicProvider{
		client: anthropic.NewClient(opts...),
		model:  m,
	}
}

func (ap *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	params := ap.buildParams(req)
	msg, err := ap.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic complete: %w", err)
	}
	return ap.toResponse(msg), nil
}

func (ap *AnthropicProvider) CompleteStream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	params := ap.buildParams(req)
	stream := ap.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		var acc anthropic.Message
		for stream.Next() {
			event := stream.Current()
			_ = acc.Accumulate(event)

			if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				if td := delta.Delta.AsTextDelta(); td.Text != "" {
					ch <- StreamChunk{Delta: td.Text}
				}
			}
		}
		if err := stream.Err(); err != nil {
			ch <- StreamChunk{Error: err, Done: true}
			return
		}
		ch <- StreamChunk{Done: true}
	}()
	return ch, nil
}

func (ap *AnthropicProvider) CompleteWithVision(ctx context.Context, req VisionRequest) (*CompletionResponse, error) {
	var imageContent anthropic.ContentBlockParamUnion
	if len(req.ImageData) > 0 {
		encoded := base64.StdEncoding.EncodeToString(req.ImageData)
		mediaType := req.MimeType
		if mediaType == "" {
			mediaType = "image/jpeg"
		}
		imageContent = anthropic.NewImageBlockBase64(mediaType, encoded)
	} else if req.ImageURL != "" {
		imageContent = anthropic.NewImageBlock(anthropic.URLImageSourceParam{
			URL: req.ImageURL,
		})
	}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(
			imageContent,
			anthropic.NewTextBlock(req.UserPrompt),
		),
	}

	params := anthropic.MessageNewParams{
		Model:     ap.model,
		Messages:  messages,
		MaxTokens: 4096,
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(req.Temperature)
	}

	msg, err := ap.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic vision: %w", err)
	}
	return ap.toResponse(msg), nil
}

func (ap *AnthropicProvider) buildParams(req CompletionRequest) anthropic.MessageNewParams {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(req.UserPrompt)),
	}

	params := anthropic.MessageNewParams{
		Model:     ap.model,
		Messages:  messages,
		MaxTokens: 4096,
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(req.Temperature)
	}

	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			var inputSchema anthropic.ToolInputSchemaParam
			if len(t.Schema) > 0 {
				_ = json.Unmarshal(t.Schema, &inputSchema)
			}
			params.Tools = append(params.Tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        t.Name,
					Description: param.NewOpt(t.Description),
					InputSchema: inputSchema,
				},
			})
		}
	}

	return params
}

func (ap *AnthropicProvider) toResponse(msg *anthropic.Message) *CompletionResponse {
	resp := &CompletionResponse{
		StopReason: string(msg.StopReason),
		Usage: Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}

	var toolCalls []ToolCall
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			tb := block.AsText()
			resp.Content += tb.Text
		case "tool_use":
			tu := block.AsToolUse()
			toolCalls = append(toolCalls, ToolCall{
				ID:    tu.ID,
				Name:  tu.Name,
				Input: tu.Input,
			})
		}
	}
	resp.ToolCalls = toolCalls

	return resp
}
