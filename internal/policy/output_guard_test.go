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
