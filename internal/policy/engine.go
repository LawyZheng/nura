package policy

import "context"

// PolicyEngine is the unified interface for the safety/policy system.
type PolicyEngine interface {
	ClassifyRisk(ctx context.Context, intent Intent) (RiskLevel, error)
	EnforcePolicy(ctx context.Context, risk RiskLevel, plan AgentPlan) (PolicyDecision, error)
	GuardOutput(ctx context.Context, output AgentOutput) (GuardedOutput, error)
}

// Engine is the default PolicyEngine implementation combining classifier,
// medical policy, and output guard.
type Engine struct {
	classifier *RiskClassifier
	policy     *MedicalPolicy
	guard      *OutputGuard
}

func NewEngine() *Engine {
	return &Engine{
		classifier: NewRiskClassifier(),
		policy:     NewMedicalPolicy(),
		guard:      NewOutputGuard(),
	}
}

func (e *Engine) ClassifyRisk(ctx context.Context, intent Intent) (RiskLevel, error) {
	return e.classifier.ClassifyRisk(ctx, intent)
}

func (e *Engine) EnforcePolicy(ctx context.Context, risk RiskLevel, plan AgentPlan) (PolicyDecision, error) {
	return e.policy.EnforcePolicy(ctx, risk, plan)
}

func (e *Engine) GuardOutput(ctx context.Context, output AgentOutput) (GuardedOutput, error) {
	return e.guard.GuardOutput(ctx, output)
}
