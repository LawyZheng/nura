package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/LawyZheng/nura/internal/tools"
)

// ToolRegistry manages tool registration and execution with trace recording.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]tools.Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]tools.Tool)}
}

// Register adds a tool to the registry. Returns an error if a tool with the
// same name is already registered.
func (tr *ToolRegistry) Register(t tools.Tool) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if _, exists := tr.tools[t.Name()]; exists {
		return fmt.Errorf("tool %q already registered", t.Name())
	}
	tr.tools[t.Name()] = t
	return nil
}

// Get retrieves a tool by name.
func (tr *ToolRegistry) Get(name string) (tools.Tool, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	t, ok := tr.tools[name]
	return t, ok
}

// List returns all registered tool names.
func (tr *ToolRegistry) List() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	names := make([]string, 0, len(tr.tools))
	for name := range tr.tools {
		names = append(names, name)
	}
	return names
}

// ToolDefs returns tool definitions suitable for LLM tool calling.
func (tr *ToolRegistry) ToolDefs() []ToolDef {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	defs := make([]ToolDef, 0, len(tr.tools))
	for _, t := range tr.tools {
		defs = append(defs, ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return defs
}

// ToolDef describes a tool for the LLM.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// Execute runs a tool by name, recording the execution in the tracer.
func (tr *ToolRegistry) Execute(ctx context.Context, tracer *Tracer, toolName string, input json.RawMessage) (tools.ToolResult, error) {
	tool, ok := tr.Get(toolName)
	if !ok {
		return tools.ToolResult{}, fmt.Errorf("tool %q not found", toolName)
	}

	stepIdx := tracer.StartStep("tool_call", toolName, string(input))
	result, err := tool.Execute(ctx, input)
	if err != nil {
		tracer.EndStep(stepIdx, "", err.Error())
		return result, err
	}

	output, _ := json.Marshal(result)
	tracer.EndStep(stepIdx, string(output), "")
	return result, nil
}
