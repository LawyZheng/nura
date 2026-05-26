package policy

import (
	"context"
	"strings"
)

// RiskLevel represents the medical risk classification of a user intent.
type RiskLevel int

const (
	RiskLow       RiskLevel = iota // indicator meanings, report terminology
	RiskMedium                     // diet, lifestyle, symptom trends
	RiskHigh                       // medication, dosage changes, HP eradication failure
	RiskEmergency                  // black stool, hematemesis, severe pain, syncope
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// Intent captures the user's parsed intent for risk classification.
type Intent struct {
	RawMessage string   `json:"raw_message"`
	Category   string   `json:"category,omitempty"` // chat / report / medication / emergency / ...
	Keywords   []string `json:"keywords,omitempty"`
}

// emergencyKeywords triggers immediate "seek medical attention" response.
var emergencyKeywords = []string{
	"黑便", "血便", "呕血", "吐血", "剧烈腹痛", "剧痛",
	"晕厥", "晕倒", "昏倒", "持续呕吐", "无法进食", "腹肌紧张",
	"black stool", "bloody stool", "hematemesis", "vomiting blood",
	"severe pain", "syncope", "fainting",
}

var highRiskKeywords = []string{
	"用药", "停药", "换药", "剂量", "处方", "药物",
	"四联", "三联", "根治", "耐药", "抗生素",
	"medication", "dosage", "prescription", "drug interaction",
	"eradication", "antibiotic",
}

var mediumRiskKeywords = []string{
	"饮食", "吃什么", "能不能吃", "生活", "运动", "睡眠",
	"症状", "趋势", "变化", "好转", "恶化",
	"diet", "eat", "food", "lifestyle", "exercise",
	"symptom", "trend", "worse", "better",
}

// RiskClassifier classifies user intent into risk levels.
type RiskClassifier struct{}

func NewRiskClassifier() *RiskClassifier {
	return &RiskClassifier{}
}

// ClassifyRisk determines the risk level based on intent keywords.
func (rc *RiskClassifier) ClassifyRisk(_ context.Context, intent Intent) (RiskLevel, error) {
	msg := strings.ToLower(intent.RawMessage)

	for _, kw := range emergencyKeywords {
		if strings.Contains(msg, strings.ToLower(kw)) {
			return RiskEmergency, nil
		}
	}

	for _, kw := range highRiskKeywords {
		if strings.Contains(msg, strings.ToLower(kw)) {
			return RiskHigh, nil
		}
	}

	for _, kw := range mediumRiskKeywords {
		if strings.Contains(msg, strings.ToLower(kw)) {
			return RiskMedium, nil
		}
	}

	return RiskLow, nil
}
