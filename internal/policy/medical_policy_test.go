package policy

import (
	"context"
	"testing"
)

func TestEnforcePolicy_Emergency(t *testing.T) {
	mp := NewMedicalPolicy()
	decision, err := mp.EnforcePolicy(context.Background(), RiskEmergency, AgentPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Error("expected emergency to be not allowed")
	}
	if decision.EmergencyMessage == "" {
		t.Error("expected emergency message")
	}
}

func TestEnforcePolicy_High(t *testing.T) {
	mp := NewMedicalPolicy()
	decision, err := mp.EnforcePolicy(context.Background(), RiskHigh, AgentPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Error("expected high risk to be allowed with guardrails")
	}
	if !decision.RequireRAG {
		t.Error("expected high risk to require RAG")
	}
	if !decision.RequireDisclaimer {
		t.Error("expected high risk to require disclaimer")
	}
}

func TestEnforcePolicy_Medium(t *testing.T) {
	mp := NewMedicalPolicy()
	decision, err := mp.EnforcePolicy(context.Background(), RiskMedium, AgentPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Error("expected medium risk to be allowed")
	}
	if decision.RequireRAG {
		t.Error("expected medium risk to not require RAG")
	}
	if !decision.RequireDisclaimer {
		t.Error("expected medium risk to require disclaimer")
	}
}

func TestEnforcePolicy_Low(t *testing.T) {
	mp := NewMedicalPolicy()
	decision, err := mp.EnforcePolicy(context.Background(), RiskLow, AgentPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Error("expected low risk to be allowed")
	}
	if decision.RequireRAG {
		t.Error("expected low risk to not require RAG")
	}
}
