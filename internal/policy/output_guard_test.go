package policy

import (
	"context"
	"strings"
	"testing"
)

func TestGuardOutput_Emergency(t *testing.T) {
	og := NewOutputGuard()
	guarded, err := og.GuardOutput(context.Background(), AgentOutput{
		Content:   "test content",
		RiskLevel: RiskEmergency,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !guarded.Labels.IsEmergency {
		t.Error("expected emergency label")
	}
	if !strings.Contains(guarded.Content, "立即就医") {
		t.Error("expected emergency content to contain '立即就医'")
	}
}

func TestGuardOutput_Normal(t *testing.T) {
	og := NewOutputGuard()
	guarded, err := og.GuardOutput(context.Background(), AgentOutput{
		Content:    "你的血红蛋白偏低",
		RiskLevel:  RiskLow,
		Sources:    []string{"血常规报告 2024-03-01"},
		Confidence: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !guarded.Labels.HasDisclaimer {
		t.Error("expected disclaimer label")
	}
	if !guarded.Labels.HasSources {
		t.Error("expected sources label")
	}
	if !strings.Contains(guarded.Content, "参考来源") {
		t.Error("expected content to contain source references")
	}
	if !strings.Contains(guarded.Content, "仅供健康参考") {
		t.Error("expected content to contain disclaimer")
	}
}

func TestGuardOutput_DefaultConfidence(t *testing.T) {
	og := NewOutputGuard()
	guarded, err := og.GuardOutput(context.Background(), AgentOutput{
		Content:   "some content",
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if guarded.Confidence != "medium" {
		t.Errorf("expected default confidence 'medium', got %q", guarded.Confidence)
	}
}

func TestGuardOutput_SanitizesUnsafePrescriptionPhrases(t *testing.T) {
	og := NewOutputGuard()

	cases := []struct {
		name    string
		content string
		phrase  string
	}{
		{"建议服用", "您的报告显示溃疡活动期，建议服用奥美拉唑进行治疗。", "建议服用"},
		{"处方", "处方建议：阿莫西林+克拉霉素。", "处方"},
		{"自行用药", "可以自行用药缓解症状。", "自行用药"},
		{"根治治疗", "HP 阳性，建议进行根治治疗。", "根治治疗"},
		{"停药", "症状缓解后可考虑停药。", "停药"},
		{"加量", "效果不佳时可加量服用。", "加量"},
		{"减量", "好转后可以减量。", "减量"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guarded, err := og.GuardOutput(context.Background(), AgentOutput{
				Content:   tc.content,
				RiskLevel: RiskMedium,
			})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(guarded.Content, tc.phrase) {
				t.Errorf("guarded output still contains unsafe phrase %q", tc.phrase)
			}
			if !strings.Contains(guarded.Content, "请咨询医生") {
				t.Error("expected safe replacement wording containing '请咨询医生'")
			}
			if !guarded.Labels.HasSanitizedContent {
				t.Error("expected HasSanitizedContent label to be true")
			}
		})
	}
}

func TestGuardOutput_PreservesNonUnsafeContent(t *testing.T) {
	og := NewOutputGuard()
	safe := "您的血红蛋白偏低，建议咨询医生进一步评估。"
	guarded, err := og.GuardOutput(context.Background(), AgentOutput{
		Content:   safe,
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(guarded.Content, "血红蛋白偏低") {
		t.Error("expected safe content to be preserved")
	}
	if guarded.Labels.HasSanitizedContent {
		t.Error("expected HasSanitizedContent to be false for safe content")
	}
}
