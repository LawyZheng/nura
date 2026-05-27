package tools

import (
	"context"
	"encoding/json"
)

// ToolResult is the output of a tool execution.
type ToolResult struct {
	Data     json.RawMessage `json:"data,omitempty"`
	Message  string          `json:"message,omitempty"`
	Error    string          `json:"error,omitempty"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

// Tool defines the interface every agent tool must implement.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
