package policy

import (
	"context"
	"testing"
)

func TestEngine_ImplementsPolicyEngine(t *testing.T) {
	var _ PolicyEngine = NewEngine()
}

func TestEngine_EndToEnd(t *testing.T) {
	engine := NewEngine()
	ctx := context.Background()

	// Emergency case.
	risk, err := engine.ClassifyRisk(ctx, Intent{RawMessage: "我在呕血"})
	if err != nil {
		t.Fatal(err)
	}
	if risk != RiskEmergency {
		t.Fatalf("expected emergency, got %v", risk)
	}

	decision, err := engine.EnforcePolicy(ctx, risk, AgentPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Error("emergency should not be allowed")
	}

	guarded, err := engine.GuardOutput(ctx, AgentOutput{
		Content:   decision.EmergencyMessage,
		RiskLevel: risk,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !guarded.Labels.IsEmergency {
		t.Error("expected emergency label in guarded output")
	}
}
