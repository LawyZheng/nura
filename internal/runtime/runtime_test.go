package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
	"github.com/LawyZheng/nura/internal/tools"
)

func newTestRuntime(t *testing.T) *AgentRuntime {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	return NewAgentRuntime(llm, pe, s)
}

func TestAgentRuntime_Run_LowRisk(t *testing.T) {
	rt := newTestRuntime(t)

	resp, err := rt.Run(context.Background(), RunRequest{
		UserMessage: "什么是 DOB 值",
		TaskHint:    "chat",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.UserReply == "" {
		t.Error("expected non-empty reply")
	}
	if resp.TraceID == "" {
		t.Error("expected non-empty trace ID")
	}
	if !resp.SafetyLabels.HasDisclaimer {
		t.Error("expected disclaimer in safety labels")
	}
}

func TestAgentRuntime_Run_Emergency(t *testing.T) {
	rt := newTestRuntime(t)

	resp, err := rt.Run(context.Background(), RunRequest{
		UserMessage: "我在呕血怎么办",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.SafetyLabels.IsEmergency {
		t.Error("expected emergency label")
	}
	if resp.TraceID == "" {
		t.Error("expected trace ID even for emergency")
	}
}

func TestAgentRuntime_Trace(t *testing.T) {
	rt := newTestRuntime(t)

	resp, err := rt.Run(context.Background(), RunRequest{
		UserMessage: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	trace, ok := rt.Traces().Get(resp.TraceID)
	if !ok {
		t.Fatal("trace not found")
	}
	if trace.ID != resp.TraceID {
		t.Errorf("trace ID mismatch: %q vs %q", trace.ID, resp.TraceID)
	}
	if len(trace.Steps) == 0 {
		t.Error("expected trace steps")
	}
	if trace.FinishedAt == nil {
		t.Error("expected trace to be finished")
	}
}

func TestAgentRuntime_GetTrace_NotFound(t *testing.T) {
	rt := newTestRuntime(t)
	_, ok := rt.GetTrace("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

// --- ToolRegistry tests ---

type mockTool struct {
	name string
}

func (m *mockTool) Name() string            { return m.name }
func (m *mockTool) Description() string     { return "mock tool" }
func (m *mockTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (tools.ToolResult, error) {
	return tools.ToolResult{Message: "executed"}, nil
}

func TestToolRegistry_RegisterAndGet(t *testing.T) {
	reg := NewToolRegistry()
	tool := &mockTool{name: "test_tool"}

	if err := reg.Register(tool); err != nil {
		t.Fatal(err)
	}

	got, ok := reg.Get("test_tool")
	if !ok {
		t.Fatal("tool not found")
	}
	if got.Name() != "test_tool" {
		t.Errorf("name = %q, want 'test_tool'", got.Name())
	}
}

func TestToolRegistry_DuplicateRegister(t *testing.T) {
	reg := NewToolRegistry()
	tool := &mockTool{name: "dup"}

	if err := reg.Register(tool); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(tool); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestToolRegistry_Execute(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&mockTool{name: "exec_test"})

	tracer := NewTracer("test input")
	result, err := reg.Execute(context.Background(), tracer, "exec_test", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "executed" {
		t.Errorf("message = %q, want 'executed'", result.Message)
	}

	run := tracer.Run()
	if len(run.Steps) != 1 {
		t.Errorf("expected 1 trace step, got %d", len(run.Steps))
	}
}

func TestToolRegistry_ExecuteNotFound(t *testing.T) {
	reg := NewToolRegistry()
	tracer := NewTracer("test")
	_, err := reg.Execute(context.Background(), tracer, "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestToolRegistry_List(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(&mockTool{name: "a"})
	reg.Register(&mockTool{name: "b"})

	names := reg.List()
	if len(names) != 2 {
		t.Errorf("expected 2 tools, got %d", len(names))
	}
}

// --- Tracer tests ---

func TestTracer_Flow(t *testing.T) {
	tracer := NewTracer("test input")

	idx := tracer.StartStep("tool_call", "test_tool", "input data")
	tracer.EndStep(idx, "output data", "")

	run := tracer.Finish("final output", "")
	if run.ID == "" {
		t.Error("expected non-empty ID")
	}
	if run.Input != "test input" {
		t.Errorf("input = %q, want 'test input'", run.Input)
	}
	if run.Output != "final output" {
		t.Errorf("output = %q, want 'final output'", run.Output)
	}
	if len(run.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(run.Steps))
	}
	if run.Steps[0].Output != "output data" {
		t.Errorf("step output = %q, want 'output data'", run.Steps[0].Output)
	}
	if run.FinishedAt == nil {
		t.Error("expected finished timestamp")
	}
}

// --- TraceStore tests ---

func TestTraceStore_SaveAndGet(t *testing.T) {
	ts := NewTraceStore()
	tracer := NewTracer("test")
	run := tracer.Finish("done", "")

	ts.Save(run)

	got, ok := ts.Get(run.ID)
	if !ok {
		t.Fatal("trace not found")
	}
	if got.Output != "done" {
		t.Errorf("output = %q, want 'done'", got.Output)
	}
}
