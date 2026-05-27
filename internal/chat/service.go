package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/LawyZheng/nura/internal/memory"
	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/rag"
	"github.com/LawyZheng/nura/internal/store"
)

const historyLimit = 10

type ChatReply struct {
	Content    string   `json:"content"`
	Sources    []string `json:"sources,omitempty"`
	Confidence string   `json:"confidence"`
	RiskLevel  string   `json:"risk_level"`
	Disclaimer string   `json:"disclaimer"`
}

type Service struct {
	llm       providers.LLMProvider
	policy    policy.PolicyEngine
	knowledge *rag.KnowledgeStore
	store     *store.Store
	ctxBuild  *memory.ContextBuilder
}

func NewService(llm providers.LLMProvider, pe policy.PolicyEngine, ks *rag.KnowledgeStore, s *store.Store) *Service {
	return &Service{
		llm:       llm,
		policy:    pe,
		knowledge: ks,
		store:     s,
		ctxBuild:  memory.NewContextBuilder(s),
	}
}

func (s *Service) HandleMessage(ctx context.Context, patientID int, message string) (*ChatReply, error) {
	s.store.InsertChatMessage(&model.ChatMessage{
		PatientID: patientID,
		Role:      "user",
		Content:   message,
	})

	intent := policy.Intent{RawMessage: message, Category: "chat"}
	risk, err := s.policy.ClassifyRisk(ctx, intent)
	if err != nil {
		return nil, fmt.Errorf("classify risk: %w", err)
	}

	plan := policy.AgentPlan{Intent: intent}
	decision, err := s.policy.EnforcePolicy(ctx, risk, plan)
	if err != nil {
		return nil, fmt.Errorf("enforce policy: %w", err)
	}

	if !decision.Allowed && decision.EmergencyMessage != "" {
		reply := &ChatReply{
			Content:    decision.EmergencyMessage,
			RiskLevel:  risk.String(),
			Disclaimer: "检测到可能的紧急健康状况。请立即就医或拨打 120 急救电话。",
		}
		s.store.InsertChatMessage(&model.ChatMessage{
			PatientID: patientID,
			Role:      "assistant",
			Content:   reply.Content,
			RiskLevel: reply.RiskLevel,
		})
		return reply, nil
	}

	var ragResults []rag.SearchResult
	if decision.RequireRAG && s.knowledge != nil {
		ragResults = s.knowledge.Search(message)
	}

	systemPrompt := s.buildPrompt(patientID, ragResults)

	resp, err := s.llm.Complete(ctx, providers.CompletionRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   message,
		Temperature:  0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	var sources []string
	seen := make(map[string]bool)
	for _, r := range ragResults {
		if !seen[r.Source] {
			seen[r.Source] = true
			sources = append(sources, r.Source)
		}
	}

	output := policy.AgentOutput{
		Content:    resp.Content,
		Sources:    sources,
		Confidence: "medium",
		RiskLevel:  risk,
	}
	guarded, err := s.policy.GuardOutput(ctx, output)
	if err != nil {
		return nil, fmt.Errorf("guard output: %w", err)
	}

	reply := &ChatReply{
		Content:    guarded.Content,
		Sources:    sources,
		Confidence: guarded.Confidence,
		RiskLevel:  risk.String(),
		Disclaimer: guarded.Disclaimer,
	}

	s.store.InsertChatMessage(&model.ChatMessage{
		PatientID:  patientID,
		Role:       "assistant",
		Content:    reply.Content,
		Sources:    model.StringList(sources),
		Confidence: reply.Confidence,
		RiskLevel:  reply.RiskLevel,
	})

	return reply, nil
}

func (s *Service) buildPrompt(patientID int, ragResults []rag.SearchResult) string {
	var sb strings.Builder

	sb.WriteString(`你是知愈（Nura），一个专注于消化性溃疡管理的健康助手。

## 规则
- 不做诊断，不建议具体药物和剂量
- 发现危险信号（黑便、呕血、剧烈腹痛）时强烈建议立即就医
- 用通俗语言回答，避免不必要的专业术语
- 回答简洁，控制在 200 字以内
- 所有回答以健康参考为目的，不构成医疗建议
`)

	agentCtx, err := s.ctxBuild.Build(patientID, "chat")
	if err == nil && agentCtx.PatientSummary != "" {
		sb.WriteString("\n")
		sb.WriteString(agentCtx.PatientSummary)
	}

	history, _ := s.store.ListRecentChatMessages(patientID, historyLimit)
	if len(history) > 0 {
		sb.WriteString("\n## 近期对话\n")
		for _, msg := range history {
			switch msg.Role {
			case "user":
				sb.WriteString(fmt.Sprintf("[用户]: %s\n", msg.Content))
			case "assistant":
				sb.WriteString(fmt.Sprintf("[知愈]: %s\n", msg.Content))
			}
		}
	}

	if len(ragResults) > 0 {
		sb.WriteString("\n## 参考知识（请基于以下信息回答并标注来源）\n")
		for _, r := range ragResults {
			sb.WriteString(fmt.Sprintf("来源：%s\n%s\n\n", r.Source, r.Content))
		}
	}

	return sb.String()
}
