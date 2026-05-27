package policy

import (
	"context"
	"testing"
)

func TestClassifyRisk_Emergency(t *testing.T) {
	rc := NewRiskClassifier()
	cases := []struct {
		msg  string
		want RiskLevel
	}{
		{"我拉了黑便怎么办", RiskEmergency},
		{"刚才呕血了", RiskEmergency},
		{"剧烈腹痛受不了", RiskEmergency},
		{"我晕倒了", RiskEmergency},
		{"持续呕吐无法进食", RiskEmergency},
		{"I have black stool", RiskEmergency},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			got, err := rc.ClassifyRisk(context.Background(), Intent{RawMessage: tc.msg})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("ClassifyRisk(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

func TestClassifyRisk_High(t *testing.T) {
	rc := NewRiskClassifier()
	cases := []string{
		"我能不能停药",
		"四联药吃完了需要换药吗",
		"克拉霉素剂量是多少",
		"HP根治失败怎么办",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			got, err := rc.ClassifyRisk(context.Background(), Intent{RawMessage: msg})
			if err != nil {
				t.Fatal(err)
			}
			if got != RiskHigh {
				t.Errorf("ClassifyRisk(%q) = %v, want high", msg, got)
			}
		})
	}
}

func TestClassifyRisk_Medium(t *testing.T) {
	rc := NewRiskClassifier()
	cases := []string{
		"我能不能吃辣",
		"最近症状有好转吗",
		"运动对溃疡有影响吗",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			got, err := rc.ClassifyRisk(context.Background(), Intent{RawMessage: msg})
			if err != nil {
				t.Fatal(err)
			}
			if got != RiskMedium {
				t.Errorf("ClassifyRisk(%q) = %v, want medium", msg, got)
			}
		})
	}
}

func TestClassifyRisk_Low(t *testing.T) {
	rc := NewRiskClassifier()
	got, err := rc.ClassifyRisk(context.Background(), Intent{RawMessage: "什么是 DOB 值"})
	if err != nil {
		t.Fatal(err)
	}
	if got != RiskLow {
		t.Errorf("ClassifyRisk = %v, want low", got)
	}
}

func TestRiskLevel_String(t *testing.T) {
	cases := []struct {
		r    RiskLevel
		want string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskEmergency, "emergency"},
		{RiskLevel(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.r.String(); got != tc.want {
			t.Errorf("RiskLevel(%d).String() = %q, want %q", tc.r, got, tc.want)
		}
	}
}
