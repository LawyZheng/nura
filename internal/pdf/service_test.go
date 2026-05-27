package pdf

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// SYNTHETIC DATA - not real patient information
func seedTestPatient(t *testing.T, s *store.Store) int {
	t.Helper()

	id, err := s.CreatePatientProfile(&model.PatientProfile{
		Name:      "张三（合成）",
		Gender:    "male",
		BirthDate: "1985-06-15",
		Height:    175,
		Weight:    70,
		Allergies: model.StringList{"青霉素"},
	})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}

	if _, err := s.InsertDiagnosis(&model.Diagnosis{
		PatientID:     id,
		DiagnosisDate: "2026-01-10",
		Condition:     "十二指肠球部溃疡",
		Note:          "SYNTHETIC - HP阳性",
	}); err != nil {
		t.Fatalf("insert diagnosis: %v", err)
	}

	if _, err := s.InsertMedication(&model.Medication{
		PatientID: id,
		Name:      "奥美拉唑",
		Dosage:    "20mg",
		Frequency: "BID",
		IsActive:  true,
	}); err != nil {
		t.Fatalf("insert medication: %v", err)
	}

	refLow := 4.0
	refHigh := 10.0
	if _, err := s.InsertIndicator(&model.MedicalIndicator{
		PatientID:        id,
		ReportID:         0,
		Category:         "blood",
		IndicatorName:    "WBC",
		IndicatorNameCN:  "白细胞",
		Value:            "12.5",
		Unit:             "10^9/L",
		ReferenceLow:     &refLow,
		ReferenceHigh:    &refHigh,
		IsAbnormal:       true,
		AbnormalDirection: "high",
		MeasuredAt:       "2026-05-20",
	}); err != nil {
		t.Fatalf("insert indicator: %v", err)
	}

	painScore := 5
	if _, err := s.InsertSymptomLog(&model.SymptomLog{
		PatientID:    id,
		PainScore:    &painScore,
		PainLocation: "upper_abdomen",
		PainTiming:   "fasting",
		RecordedAt:   time.Now().Add(-24 * time.Hour),
	}); err != nil {
		t.Fatalf("insert symptom: %v", err)
	}

	if _, err := s.InsertAIInsight(&model.AIInsight{
		PatientID:  id,
		InsightType: "trend_7d",
		Content:    "SYNTHETIC: 近7天症状整体平稳，腹痛评分维持在4-6分。建议继续规律用药并避免刺激性食物。",
	}); err != nil {
		t.Fatalf("insert insight: %v", err)
	}

	return id
}

func TestGenerateSummary_ValidPatient(t *testing.T) {
	s := newTestStore(t)
	pid := seedTestPatient(t, s)

	svc := NewService(s)
	pdfBytes, err := svc.GenerateSummary(pid)
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}
}

func TestGenerateSummary_PDFHeader(t *testing.T) {
	s := newTestStore(t)
	pid := seedTestPatient(t, s)

	svc := NewService(s)
	pdfBytes, err := svc.GenerateSummary(pid)
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if len(pdfBytes) < 4 || string(pdfBytes[:5]) != "%PDF-" {
		t.Fatalf("expected PDF header %%PDF-, got %q", string(pdfBytes[:min(5, len(pdfBytes))]))
	}
}

func TestGenerateSummary_PatientNotFound(t *testing.T) {
	s := newTestStore(t)

	svc := NewService(s)
	_, err := svc.GenerateSummary(99999)
	if err == nil {
		t.Fatal("expected error for nonexistent patient")
	}
}
