package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReportType_Values(t *testing.T) {
	types := []ReportType{
		ReportGastroscopy, ReportHPBreath, ReportHPAntibody,
		ReportBloodRoutine, ReportLiverFunction, ReportKidneyFunction,
		ReportStoolRoutine, ReportStoolOccultBlood, ReportGeneralCheckup,
		ReportUnknown,
	}
	seen := make(map[ReportType]bool)
	for _, rt := range types {
		if seen[rt] {
			t.Errorf("duplicate report type: %s", rt)
		}
		seen[rt] = true
		if string(rt) == "" {
			t.Error("empty report type string")
		}
	}
}

func TestHealthReport_JSON(t *testing.T) {
	r := HealthReport{
		ID:         1,
		ReportType: ReportGastroscopy,
		ReportDate: "2024-03-01",
		RawText:    "test",
		CreatedAt:  time.Now(),
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	var r2 HealthReport
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatal(err)
	}
	if r2.ReportType != ReportGastroscopy {
		t.Errorf("report type = %q, want 'gastroscopy'", r2.ReportType)
	}
}

func TestTraceRun_JSON(t *testing.T) {
	now := time.Now()
	tr := TraceRun{
		ID:        "test-id",
		StartedAt: now,
		Input:     "test input",
		Steps: []TraceStep{
			{Index: 0, StepType: "tool_call", Name: "test", StartedAt: now, DurationMs: 100},
		},
	}
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}

	var tr2 TraceRun
	if err := json.Unmarshal(data, &tr2); err != nil {
		t.Fatal(err)
	}
	if tr2.ID != "test-id" {
		t.Errorf("id = %q, want 'test-id'", tr2.ID)
	}
	if len(tr2.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(tr2.Steps))
	}
}

func TestPatientProfile_Allergies(t *testing.T) {
	p := PatientProfile{
		Allergies: StringList{"penicillin", "aspirin"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	var p2 PatientProfile
	if err := json.Unmarshal(data, &p2); err != nil {
		t.Fatal(err)
	}
	if len(p2.Allergies) != 2 {
		t.Errorf("expected 2 allergies, got %d", len(p2.Allergies))
	}
}

func TestStringList_ValueAndScan(t *testing.T) {
	s := StringList{"a", "b", "c"}
	v, err := s.Value()
	if err != nil {
		t.Fatal(err)
	}
	str, ok := v.(string)
	if !ok {
		t.Fatal("expected string from Value()")
	}

	var s2 StringList
	if err := s2.Scan(str); err != nil {
		t.Fatal(err)
	}
	if len(s2) != 3 {
		t.Errorf("expected 3 items, got %d", len(s2))
	}
}

func TestStringList_ScanNil(t *testing.T) {
	var s StringList
	if err := s.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Errorf("expected nil, got %v", s)
	}
}
