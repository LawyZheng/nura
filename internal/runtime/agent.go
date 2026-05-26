package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LawyZheng/nura/internal/memory"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

// RunRequest is the input to an agent run.
type RunRequest struct {
	UserMessage string `json:"user_message"`
	TaskHint    string `json:"task_hint,omitempty"` // chat / report_ingest / visit_summary
	PatientID   string `json:"patient_id,omitempty"`
}

// RunResponse is the structured output of an agent run.
type RunResponse struct {
	Result         map[string]any      `json:"result,omitempty"`
	UserReply      string              `json:"user_facing_reply"`
	MemoryUpdates  []string            `json:"memory_updates,omitempty"`
	SafetyLabels   policy.SafetyLabels `json:"safety_labels"`
	TraceID        string              `json:"trace_id"`
}

// AgentRuntime orchestrates planning, tool execution, policy enforcement,
// and memory management for a single agent run.
type AgentRuntime struct {
	llm       providers.LLMProvider
	tools     *ToolRegistry
	policy    policy.PolicyEngine
	ctxBuild  *memory.ContextBuilder
	traces    *TraceStore
}

// NewAgentRuntime creates a new runtime with all dependencies.
func NewAgentRuntime(llm providers.LLMProvider, pe policy.PolicyEngine, s *store.Store) *AgentRuntime {
	return &AgentRuntime{
		llm:      llm,
		tools:    NewToolRegistry(),
		policy:   pe,
		ctxBuild: memory.NewContextBuilder(s),
		traces:   NewTraceStore(),
	}
}

// RegisterTool adds a tool to the agent's tool registry.
func (ar *AgentRuntime) RegisterTool(t interface{ Name() string; Description() string; Schema() json.RawMessage; Execute(ctx context.Context, input json.RawMessage) (interface{ }, error) }) {
	// This is handled through the typed ToolRegistry.
}

// RegisterToolDirect adds a tools.Tool to the registry.
func (ar *AgentRuntime) RegisterToolDirect(t interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (interface{}, error)
}) error {
	// Unused — use Tools().Register() instead.
	return nil
}

// Tools returns the tool registry.
func (ar *AgentRuntime) Tools() *ToolRegistry {
	return ar.tools
}

// Traces returns the trace store.
func (ar *AgentRuntime) Traces() *TraceStore {
	return ar.traces
}

// Run executes a full agent run: classify risk → enforce policy → build context
// → plan → execute tools → guard output.
func (ar *AgentRuntime) Run(ctx context.Context, req RunRequest) (*RunResponse, error) {
	tracer := NewTracer(req.UserMessage)

	// Step 1: Classify risk.
	riskIdx := tracer.StartStep("policy_check", "classify_risk", req.UserMessage)
	intent := policy.Intent{
		RawMessage: req.UserMessage,
		Category:   req.TaskHint,
	}
	risk, err := ar.policy.ClassifyRisk(ctx, intent)
	if err != nil {
		tracer.EndStep(riskIdx, "", err.Error())
		return nil, fmt.Errorf("classify risk: %w", err)
	}
	tracer.EndStep(riskIdx, risk.String(), "")

	// Step 2: Enforce policy.
	policyIdx := tracer.StartStep("policy_check", "enforce_policy", risk.String())
	plan := policy.AgentPlan{
		Intent:    intent,
		ToolNames: ar.tools.List(),
	}
	decision, err := ar.policy.EnforcePolicy(ctx, risk, plan)
	if err != nil {
		tracer.EndStep(policyIdx, "", err.Error())
		return nil, fmt.Errorf("enforce policy: %w", err)
	}
	tracer.EndStep(policyIdx, fmt.Sprintf("allowed=%v rag=%v", decision.Allowed, decision.RequireRAG), "")

	// If emergency, return immediately with emergency message.
	if !decision.Allowed && decision.EmergencyMessage != "" {
		output := policy.AgentOutput{
			Content:   decision.EmergencyMessage,
			RiskLevel: risk,
		}
		guarded, _ := ar.policy.GuardOutput(ctx, output)
		run := tracer.Finish(guarded.Content, "")
		ar.traces.Save(run)
		return &RunResponse{
			UserReply:    guarded.Content,
			SafetyLabels: guarded.Labels,
			TraceID:      run.ID,
		}, nil
	}

	// Step 3: Build context.
	ctxIdx := tracer.StartStep("plan", "build_context", req.TaskHint)
	agentCtx, err := ar.ctxBuild.Build(req.TaskHint)
	if err != nil {
		tracer.EndStep(ctxIdx, "", err.Error())
		// Non-fatal: continue without context.
		agentCtx = &memory.AgentContext{}
	}
	tracer.EndStep(ctxIdx, fmt.Sprintf("facts=%d", len(agentCtx.RelevantFacts)), "")

	// Step 4: Call LLM with patient context.
	llmIdx := tracer.StartStep("llm_call", "complete", req.UserMessage)

	systemPrompt := buildSystemPrompt(agentCtx, decision)
	resp, err := ar.llm.Complete(ctx, providers.CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   req.UserMessage,
		Temperature:  0.1,
	})
	if err != nil {
		tracer.EndStep(llmIdx, "", err.Error())
		return nil, fmt.Errorf("llm complete: %w", err)
	}
	tracer.EndStep(llmIdx, resp.Content, "")

	// Step 5: Guard output.
	guardIdx := tracer.StartStep("guard", "output_guard", resp.Content)
	output := policy.AgentOutput{
		Content:    resp.Content,
		RiskLevel:  risk,
		Confidence: "medium",
	}
	guarded, err := ar.policy.GuardOutput(ctx, output)
	if err != nil {
		tracer.EndStep(guardIdx, "", err.Error())
		return nil, fmt.Errorf("guard output: %w", err)
	}
	tracer.EndStep(guardIdx, "guarded", "")

	// Finalize trace.
	run := tracer.Finish(guarded.Content, "")
	ar.traces.Save(run)

	return &RunResponse{
		UserReply:    guarded.Content,
		SafetyLabels: guarded.Labels,
		TraceID:      run.ID,
	}, nil
}

// GetTrace retrieves a completed trace by ID.
func (ar *AgentRuntime) GetTrace(id string) (*RunResponse, bool) {
	run, ok := ar.traces.Get(id)
	if !ok {
		return nil, false
	}
	return &RunResponse{
		UserReply: run.Output,
		TraceID:   run.ID,
	}, true
}

func buildSystemPrompt(agentCtx *memory.AgentContext, decision policy.PolicyDecision) string {
	prompt := `你是知愈（Nura），一个专注于消化性溃疡管理的健康助手。

## 规则
- 不做诊断，不建议具体药物和剂量
- 发现危险信号（黑便、呕血、剧烈腹痛）时强烈建议立即就医
- 用通俗语言回答，避免不必要的专业术语
- 回答简洁，控制在 200 字以内
- 所有回答以健康参考为目的，不构成医疗建议
`

	if agentCtx.PatientSummary != "" {
		prompt += "\n" + agentCtx.PatientSummary
	}

	if decision.RequireRAG {
		prompt += "\n注意：此问题涉及用药/治疗相关内容，请基于医学指南回答并标注来源。\n"
	}

	if decision.RequireDisclaimer {
		prompt += "\n请在回答末尾附上「以上内容仅供健康参考，不构成医疗建议。如有疑问请咨询专业医生。」\n"
	}

	return prompt
}
