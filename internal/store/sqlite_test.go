package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStore_PatientProfile(t *testing.T) {
	s := newTestStore(t)

	err := s.UpsertPatientProfile(&model.PatientProfile{
		Name:      "Test Patient",
		Gender:    "male",
		BirthDate: "1990-01-01",
		Height:    175,
		Weight:    70,
		Allergies: model.StringList{"penicillin"},
	})
	if err != nil {
		t.Fatal(err)
	}

	p, err := s.GetPatientProfile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Test Patient" {
		t.Errorf("name = %q, want 'Test Patient'", p.Name)
	}
	if p.Gender != "male" {
		t.Errorf("gender = %q, want 'male'", p.Gender)
	}
	if len(p.Allergies) != 1 || p.Allergies[0] != "penicillin" {
		t.Errorf("allergies = %v, want [penicillin]", p.Allergies)
	}

	// Update.
	err = s.UpsertPatientProfile(&model.PatientProfile{
		Name:   "Updated Patient",
		Weight: 72,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.GetPatientProfile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Updated Patient" {
		t.Errorf("updated name = %q, want 'Updated Patient'", p.Name)
	}
}

func TestStore_HealthReport(t *testing.T) {
	s := newTestStore(t)

	id, err := s.InsertHealthReport(&model.HealthReport{
		ReportType: model.ReportGastroscopy,
		ReportDate: "2024-03-01",
		RawText:    "test report content",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	r, err := s.GetHealthReport(id)
	if err != nil {
		t.Fatal(err)
	}
	if r.ReportType != model.ReportGastroscopy {
		t.Errorf("report_type = %q, want gastroscopy", r.ReportType)
	}
	if r.RawText != "test report content" {
		t.Errorf("raw_text = %q, want 'test report content'", r.RawText)
	}

	// Update processed.
	err = s.UpdateHealthReportProcessed(id, "gastroscopy", "AI summary")
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.GetHealthReport(id)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsProcessed {
		t.Error("expected is_processed=true after update")
	}
}

func TestStore_MedicalIndicator(t *testing.T) {
	s := newTestStore(t)

	reportID, _ := s.InsertHealthReport(&model.HealthReport{
		ReportType: model.ReportBloodRoutine,
		ReportDate: "2024-03-01",
		RawText:    "blood test",
	})

	refLow := 130.0
	refHigh := 175.0
	_, err := s.InsertIndicator(&model.MedicalIndicator{
		ReportID:          reportID,
		Category:          "blood",
		IndicatorName:     "hemoglobin",
		IndicatorNameCN:   "血红蛋白",
		Value:             "108",
		Unit:              "g/L",
		ReferenceLow:      &refLow,
		ReferenceHigh:     &refHigh,
		IsAbnormal:        true,
		AbnormalDirection: "low",
		MeasuredAt:        "2024-03-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	indicators, err := s.GetIndicatorsByReport(reportID)
	if err != nil {
		t.Fatal(err)
	}
	if len(indicators) != 1 {
		t.Fatalf("expected 1 indicator, got %d", len(indicators))
	}
	if indicators[0].IndicatorName != "hemoglobin" {
		t.Errorf("indicator name = %q, want 'hemoglobin'", indicators[0].IndicatorName)
	}

	// Timeline query.
	timeline, err := s.GetIndicatorTimeline("hemoglobin")
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 1 {
		t.Errorf("expected 1 timeline entry, got %d", len(timeline))
	}
}

func TestStore_Diagnosis(t *testing.T) {
	s := newTestStore(t)

	id, err := s.InsertDiagnosis(&model.Diagnosis{
		DiagnosisDate: "2024-03-01",
		Condition:     "duodenal_ulcer",
		Detail:        `{"stage":"A2","size":"0.8x0.6cm"}`,
		Note:          "confirmed by gastroscopy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	diagnoses, err := s.ListDiagnoses()
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnoses) != 1 {
		t.Fatalf("expected 1 diagnosis, got %d", len(diagnoses))
	}
	if diagnoses[0].Condition != "duodenal_ulcer" {
		t.Errorf("condition = %q, want 'duodenal_ulcer'", diagnoses[0].Condition)
	}
}

func TestStore_SymptomLog(t *testing.T) {
	s := newTestStore(t)

	painScore := 5
	_, err := s.InsertSymptomLog(&model.SymptomLog{
		PainScore:    &painScore,
		PainLocation: "upper_abdomen",
		PainTiming:   "fasting",
		StoolColor:   "normal",
		Bloating:     true,
		RecordedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	logs, err := s.ListSymptomLogs(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if *logs[0].PainScore != 5 {
		t.Errorf("pain_score = %d, want 5", *logs[0].PainScore)
	}
}

func TestStore_Medication(t *testing.T) {
	s := newTestStore(t)

	_, err := s.InsertMedication(&model.Medication{
		Name:        "Omeprazole",
		Category:    "ppi",
		Dosage:      "20mg",
		Frequency:   "bid",
		CourseStart: "2024-03-01",
		CourseEnd:   "2024-03-14",
		IsActive:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	meds, err := s.ListActiveMedications()
	if err != nil {
		t.Fatal(err)
	}
	if len(meds) != 1 {
		t.Fatalf("expected 1 medication, got %d", len(meds))
	}
	if meds[0].Name != "Omeprazole" {
		t.Errorf("name = %q, want 'Omeprazole'", meds[0].Name)
	}
}

func TestStore_MemorySummary(t *testing.T) {
	s := newTestStore(t)

	_, err := s.InsertMemorySummary(&model.MemorySummary{
		Category: "symptom_pattern",
		Content:  "Patient reports worsening pain after spicy food",
	})
	if err != nil {
		t.Fatal(err)
	}

	summaries, err := s.GetActiveMemorySummaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
}

func TestStore_MealLog(t *testing.T) {
	s := newTestStore(t)

	_, err := s.InsertMealLog(&model.MealLog{
		MealType:    "lunch",
		Content:     "rice and steamed fish",
		HasIrritant: false,
		RecordedAt:  time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStore_AIInsight(t *testing.T) {
	s := newTestStore(t)

	_, err := s.InsertAIInsight(&model.AIInsight{
		InsightType: "trend",
		Content:     "Pain scores improving over last 7 days",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStore_NewWithInvalidPath(t *testing.T) {
	_, err := New("/dev/null/nonexistent/impossible/test.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
