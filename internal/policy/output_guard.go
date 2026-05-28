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
	Content    string    `json:"content"`
	Sources    []string  `json:"sources,omitempty"`
	Confidence string    `json:"confidence,omitempty"` // high / medium / low
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
	RiskLevel           string `json:"risk_level"`
	HasDisclaimer       bool   `json:"has_disclaimer"`
	HasSources          bool   `json:"has_sources"`
	HasConfidence       bool   `json:"has_confidence"`
	IsEmergency         bool   `json:"is_emergency"`
	HasSanitizedContent bool   `json:"has_sanitized_content"`
}

// OutputGuard adds disclaimers, source citations, and confidence labels to agent output.
type OutputGuard struct{}

func NewOutputGuard() *OutputGuard {
	return &OutputGuard{}
}

// unsafePhrases lists common Chinese prescription/treatment-action wording
// that must not pass through to user-facing health content unchanged.
var unsafePhrases = []string{
	"建议服用",
	"处方",
	"自行用药",
	"根治治疗",
	"停药",
	"加量",
	"减量",
}

const safeReplacement = "涉及治疗或用药调整的内容需要由医生结合病情判断，请咨询医生。"

// sanitizeContent replaces sentences containing unsafe prescription/treatment
// phrases with a safe doctor-consult message. Returns sanitized text and
// whether any replacement was made.
func sanitizeContent(content string) (string, bool) {
	sanitized := false
	sentences := splitSentences(content)
	var result []string
	replacementUsed := false
	for _, s := range sentences {
		hit := false
		for _, phrase := range unsafePhrases {
			if strings.Contains(s, phrase) {
				hit = true
				sanitized = true
				break
			}
		}
		if hit {
			if !replacementUsed {
				result = append(result, safeReplacement)
				replacementUsed = true
			}
		} else {
			result = append(result, s)
		}
	}
	return strings.Join(result, ""), sanitized
}

// splitSentences splits Chinese/mixed text on sentence-ending punctuation,
// keeping delimiters attached to the preceding sentence.
func splitSentences(text string) []string {
	var sentences []string
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if r == '。' || r == '！' || r == '？' || r == '；' || r == '\n' {
			sentences = append(sentences, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		sentences = append(sentences, cur.String())
	}
	return sentences
}

// GuardOutput processes raw agent output and adds safety annotations.
func (og *OutputGuard) GuardOutput(_ context.Context, output AgentOutput) (GuardedOutput, error) {
	content, wasSanitized := sanitizeContent(output.Content)

	guarded := GuardedOutput{
		Content:    content,
		Sources:    output.Sources,
		Confidence: output.Confidence,
		Labels: SafetyLabels{
			RiskLevel:           output.RiskLevel.String(),
			HasSources:          len(output.Sources) > 0,
			HasConfidence:       output.Confidence != "",
			HasSanitizedContent: wasSanitized,
		},
	}

	if output.RiskLevel == RiskEmergency {
		guarded.Content = emergencyDisclaimer + "\n\n" + content
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
		sb.WriteString(content)
		sb.WriteString("\n\n---\n**参考来源：**\n")
		for i, src := range output.Sources {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, src))
		}
		guarded.Content = sb.String()
	}

	guarded.Content += "\n\n> " + defaultDisclaimer

	return guarded, nil
}
