package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
)

func seedFullPatient(t *testing.T, s *Store) int {
	t.Helper()
	pid := createTestPatient(t, s)

	s.InsertHealthReport(&model.HealthReport{
		PatientID: pid, ReportType: model.ReportGastroscopy,
		ReportDate: "2024-03-01", RawText: "gastroscopy text",
	})
	s.InsertDiagnosis(&model.Diagnosis{
		PatientID: pid, DiagnosisDate: "2024-03-01",
		Condition: "duodenal_ulcer", Note: "A2",
	})
	ps := 5
	s.InsertSymptomLog(&model.SymptomLog{
		PatientID: pid, PainScore: &ps,
		PainLocation: "upper_abdomen", RecordedAt: time.Now(),
	})
	s.InsertMealLog(&model.MealLog{
		PatientID: pid, MealType: "lunch",
		Content: "rice", RecordedAt: time.Now(),
	})
	s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Omeprazole",
		Category: "ppi", IsActive: true,
	})
	s.InsertMemorySummary(&model.MemorySummary{
		PatientID: pid, Category: "overall",
		Content: "test summary",
	})
	s.InsertAIInsight(&model.AIInsight{
		PatientID: pid, InsightType: "trend",
		Content: "improving",
	})

	return pid
}

func TestExportPatient(t *testing.T) {
	s := newTestStore(t)
	pid := seedFullPatient(t, s)

	export, err := s.ExportPatient(pid)
	if err != nil {
		t.Fatal(err)
	}

	if export.Profile == nil {
		t.Fatal("expected profile")
	}
	if len(export.Reports) != 1 {
		t.Errorf("reports = %d, want 1", len(export.Reports))
	}
	if len(export.Diagnoses) != 1 {
		t.Errorf("diagnoses = %d, want 1", len(export.Diagnoses))
	}
	if len(export.Symptoms) != 1 {
		t.Errorf("symptoms = %d, want 1", len(export.Symptoms))
	}
	if len(export.Meals) != 1 {
		t.Errorf("meals = %d, want 1", len(export.Meals))
	}
	if len(export.Medications) != 1 {
		t.Errorf("medications = %d, want 1", len(export.Medications))
	}
	if len(export.Summaries) != 1 {
		t.Errorf("summaries = %d, want 1", len(export.Summaries))
	}
	if len(export.Insights) != 1 {
		t.Errorf("insights = %d, want 1", len(export.Insights))
	}
}

func TestExportPatientJSON(t *testing.T) {
	s := newTestStore(t)
	pid := seedFullPatient(t, s)

	data, err := s.ExportPatientJSON(pid)
	if err != nil {
		t.Fatal(err)
	}

	var export PatientExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if export.Profile.Name != "Test Patient" {
		t.Errorf("name = %q, want 'Test Patient'", export.Profile.Name)
	}
}

func TestImportPatient(t *testing.T) {
	s := newTestStore(t)
	pid := seedFullPatient(t, s)

	// Export.
	export, err := s.ExportPatient(pid)
	if err != nil {
		t.Fatal(err)
	}

	// Import into a new patient.
	newPID, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Imported"})
	if err := s.ImportPatient(newPID, export); err != nil {
		t.Fatal(err)
	}

	// Verify imported data is scoped to new patient.
	reports, _ := s.ListHealthReports(newPID)
	if len(reports) != 1 {
		t.Errorf("imported reports = %d, want 1", len(reports))
	}
	diags, _ := s.ListDiagnoses(newPID)
	if len(diags) != 1 {
		t.Errorf("imported diagnoses = %d, want 1", len(diags))
	}
	meds, _ := s.ListActiveMedications(newPID)
	if len(meds) != 1 {
		t.Errorf("imported meds = %d, want 1", len(meds))
	}

	// Verify original patient data is not affected.
	origReports, _ := s.ListHealthReports(pid)
	if len(origReports) != 1 {
		t.Errorf("original reports = %d, want 1", len(origReports))
	}
}

func TestExportPatient_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.ExportPatient(999)
	if err == nil {
		t.Error("expected error for nonexistent patient")
	}
}
