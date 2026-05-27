package policy

import "context"

// AgentPlan describes what the agent intends to do in response to a user query.
type AgentPlan struct {
	Intent     Intent   `json:"intent"`
	ToolNames  []string `json:"tool_names,omitempty"`
	NeedsRAG   bool     `json:"needs_rag"`
	RawPlan    string   `json:"raw_plan,omitempty"`
}

// PolicyDecision is the output of policy enforcement on an agent plan.
type PolicyDecision struct {
	Allowed           bool     `json:"allowed"`
	RequireRAG        bool     `json:"require_rag"`
	RequireDisclaimer bool     `json:"require_disclaimer"`
	BlockedTools      []string `json:"blocked_tools,omitempty"`
	Reason            string   `json:"reason,omitempty"`
	EmergencyMessage  string   `json:"emergency_message,omitempty"`
}

// MedicalPolicy determines tool permissions, RAG requirements, and disclaimers
// based on risk level.
type MedicalPolicy struct{}

func NewMedicalPolicy() *MedicalPolicy {
	return &MedicalPolicy{}
}

// EnforcePolicy returns a PolicyDecision based on the risk level.
func (mp *MedicalPolicy) EnforcePolicy(_ context.Context, risk RiskLevel, plan AgentPlan) (PolicyDecision, error) {
	switch risk {
	case RiskEmergency:
		return PolicyDecision{
			Allowed:           false,
			RequireDisclaimer: true,
			EmergencyMessage:  "检测到紧急健康信号。请立即就医或拨打急救电话。Nura 不能替代紧急医疗救治。",
			Reason:            "emergency keywords detected",
		}, nil

	case RiskHigh:
		return PolicyDecision{
			Allowed:           true,
			RequireRAG:        true,
			RequireDisclaimer: true,
			Reason:            "high-risk medical topic requires guideline-backed response",
		}, nil

	case RiskMedium:
		return PolicyDecision{
			Allowed:           true,
			RequireRAG:        false,
			RequireDisclaimer: true,
			Reason:            "general health advice requires disclaimer",
		}, nil

	default: // RiskLow
		return PolicyDecision{
			Allowed:           true,
			RequireRAG:        false,
			RequireDisclaimer: true,
			Reason:            "informational query",
		}, nil
	}
}
