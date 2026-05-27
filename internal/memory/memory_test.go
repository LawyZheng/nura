package memory

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

func seedTestData(t *testing.T, s *store.Store) int {
	t.Helper()

	pid, err := s.CreatePatientProfile(&model.PatientProfile{
		Name:      "Test Patient",
		Gender:    "male",
		BirthDate: "1990-06-15",
		Allergies: model.StringList{"penicillin"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.InsertDiagnosis(&model.Diagnosis{
		PatientID:     pid,
		DiagnosisDate: "2024-03-01",
		Condition:     "duodenal_ulcer",
		Note:          "A2 stage",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.InsertMedication(&model.Medication{
		PatientID: pid,
		Name:      "Omeprazole",
		Category:  "ppi",
		Dosage:    "20mg",
		IsActive:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	painScore := 4
	_, err = s.InsertSymptomLog(&model.SymptomLog{
		PatientID:    pid,
		PainScore:    &painScore,
		PainLocation: "upper_abdomen",
		RecordedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.InsertHealthReport(&model.HealthReport{
		PatientID:  pid,
		ReportType: model.ReportGastroscopy,
		ReportDate: "2024-03-01",
		RawText:    "gastroscopy report text",
	})
	if err != nil {
		t.Fatal(err)
	}

	return pid
}

func TestRawEvents(t *testing.T) {
	s := newTestStore(t)
	pid := seedTestData(t, s)
	raw := NewRawEvents(s)

	reports, err := raw.ListReports(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}

	symptoms, err := raw.ListSymptoms(pid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(symptoms) != 1 {
		t.Errorf("expected 1 symptom, got %d", len(symptoms))
	}

	meds, err := raw.ListActiveMedications(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(meds) != 1 {
		t.Errorf("expected 1 medication, got %d", len(meds))
	}

	diags, err := raw.ListDiagnoses(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 {
		t.Errorf("expected 1 diagnosis, got %d", len(diags))
	}
}

func TestStructuredState_BuildState(t *testing.T) {
	s := newTestStore(t)
	pid := seedTestData(t, s)
	ss := NewStructuredState(s)

	state, err := ss.BuildState(pid)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profile == nil {
		t.Error("expected profile")
	}
	if len(state.Diagnoses) != 1 {
		t.Errorf("expected 1 diagnosis, got %d", len(state.Diagnoses))
	}
	if len(state.ActiveMedications) != 1 {
		t.Errorf("expected 1 medication, got %d", len(state.ActiveMedications))
	}
}

func TestStructuredState_Summarize(t *testing.T) {
	s := newTestStore(t)
	pid := seedTestData(t, s)
	ss := NewStructuredState(s)

	summary, err := ss.Summarize(pid)
	if err != nil {
		t.Fatal(err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(summary) < 50 {
		t.Errorf("summary too short: %d chars", len(summary))
	}
}

func TestStructuredState_EmptyStore(t *testing.T) {
	s := newTestStore(t)
	ss := NewStructuredState(s)

	state, err := ss.BuildState(999)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profile != nil {
		t.Error("expected nil profile for nonexistent patient")
	}
}

func TestContextBuilder_Build(t *testing.T) {
	s := newTestStore(t)
	pid := seedTestData(t, s)
	cb := NewContextBuilder(s)

	ctx, err := cb.Build(pid, "report")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.PatientSummary == "" {
		t.Error("expected non-empty patient summary")
	}
	if len(ctx.RelevantFacts) == 0 {
		t.Error("expected relevant facts for report hint")
	}

	ctx, err = cb.Build(pid, "medication")
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.RelevantFacts) == 0 {
		t.Error("expected relevant facts for medication hint")
	}

	ctx, err = cb.Build(pid, "chat")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.PatientSummary == "" {
		t.Error("expected patient summary even for generic hint")
	}
}

func TestDataIsolation(t *testing.T) {
	s := newTestStore(t)

	pid1 := seedTestData(t, s)
	pid2, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Other Patient"})

	raw := NewRawEvents(s)

	r1, _ := raw.ListReports(pid1)
	r2, _ := raw.ListReports(pid2)

	if len(r1) != 1 {
		t.Errorf("patient 1 should have 1 report, got %d", len(r1))
	}
	if len(r2) != 0 {
		t.Errorf("patient 2 should have 0 reports, got %d", len(r2))
	}
}
