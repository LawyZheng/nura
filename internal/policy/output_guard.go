package policy

import (
	"context"
	"fmt"
	"strings"
)

const defaultDisclaimer = "以上内容仅供健康参考，不构成医疗建议。如有疑问请咨询专业医生。"

const emergencyDisclaimer = "检测到可能的紧急健康状况。请立即就医或拨打 120 急救电话。"

// AgentOutput is the raw output from the agent before safety guard.
type AgentOutput struct {
	Content    string   `json:"content"`
	Sources    []string `json:"sources,omitempty"`
	Confidence string   `json:"confidence,omitempty"` // high / medium / low
	RiskLevel  RiskLevel `json:"risk_level"`
}

// GuardedOutput is the agent output after the safety guard has processed it.
type GuardedOutput struct {
	Content    string       `json:"content"`
	Disclaimer string       `json:"disclaimer"`
	Sources    []string     `json:"sources,omitempty"`
	Confidence string       `json:"confidence,omitempty"`
	Labels     SafetyLabels `json:"labels"`
}

// SafetyLabels are metadata labels attached to every guarded output.
type SafetyLabels struct {
	RiskLevel         string `json:"risk_level"`
	HasDisclaimer     bool   `json:"has_disclaimer"`
	HasSources        bool   `json:"has_sources"`
	HasConfidence     bool   `json:"has_confidence"`
	IsEmergency       bool   `json:"is_emergency"`
}

// OutputGuard adds disclaimers, source citations, and confidence labels to agent output.
type OutputGuard struct{}

func NewOutputGuard() *OutputGuard {
	return &OutputGuard{}
}

// GuardOutput processes raw agent output and adds safety annotations.
func (og *OutputGuard) GuardOutput(_ context.Context, output AgentOutput) (GuardedOutput, error) {
	guarded := GuardedOutput{
		Content:    output.Content,
		Sources:    output.Sources,
		Confidence: output.Confidence,
		Labels: SafetyLabels{
			RiskLevel:     output.RiskLevel.String(),
			HasSources:    len(output.Sources) > 0,
			HasConfidence: output.Confidence != "",
		},
	}

	if output.RiskLevel == RiskEmergency {
		guarded.Content = emergencyDisclaimer + "\n\n" + output.Content
		guarded.Disclaimer = emergencyDisclaimer
		guarded.Labels.IsEmergency = true
		guarded.Labels.HasDisclaimer = true
		return guarded, nil
	}

	guarded.Disclaimer = defaultDisclaimer
	guarded.Labels.HasDisclaimer = true

	if output.Confidence == "" {
		guarded.Confidence = "medium"
		guarded.Labels.HasConfidence = true
	}

	// Append sources as footnotes if present.
	if len(output.Sources) > 0 {
		var sb strings.Builder
		sb.WriteString(output.Content)
		sb.WriteString("\n\n---\n**参考来源：**\n")
		for i, src := range output.Sources {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, src))
		}
		guarded.Content = sb.String()
	}

	guarded.Content += "\n\n> " + defaultDisclaimer

	return guarded, nil
}
