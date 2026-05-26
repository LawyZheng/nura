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

func seedTestData(t *testing.T, s *store.Store) {
	t.Helper()

	err := s.UpsertPatientProfile(&model.PatientProfile{
		Name:      "Test Patient",
		Gender:    "male",
		BirthDate: "1990-06-15",
		Allergies: []string{"penicillin"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.InsertDiagnosis(&model.Diagnosis{
		DiagnosisDate: "2024-03-01",
		Condition:     "duodenal_ulcer",
		Note:          "A2 stage",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.InsertMedication(&model.Medication{
		Name:     "Omeprazole",
		Category: "ppi",
		Dosage:   "20mg",
		IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	painScore := 4
	_, err = s.InsertSymptomLog(&model.SymptomLog{
		PainScore:    &painScore,
		PainLocation: "upper_abdomen",
		RecordedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.InsertHealthReport(&model.HealthReport{
		ReportType: model.ReportGastroscopy,
		ReportDate: "2024-03-01",
		RawText:    "gastroscopy report text",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRawEvents(t *testing.T) {
	s := newTestStore(t)
	seedTestData(t, s)
	raw := NewRawEvents(s)

	reports, err := raw.ListReports()
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}

	symptoms, err := raw.ListSymptoms(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(symptoms) != 1 {
		t.Errorf("expected 1 symptom, got %d", len(symptoms))
	}

	meds, err := raw.ListActiveMedications()
	if err != nil {
		t.Fatal(err)
	}
	if len(meds) != 1 {
		t.Errorf("expected 1 medication, got %d", len(meds))
	}

	diags, err := raw.ListDiagnoses()
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 {
		t.Errorf("expected 1 diagnosis, got %d", len(diags))
	}
}

func TestStructuredState_BuildState(t *testing.T) {
	s := newTestStore(t)
	seedTestData(t, s)
	ss := NewStructuredState(s)

	state, err := ss.BuildState()
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
	seedTestData(t, s)
	ss := NewStructuredState(s)

	summary, err := ss.Summarize()
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

	state, err := ss.BuildState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Profile != nil {
		t.Error("expected nil profile for empty store")
	}
}

func TestContextBuilder_Build(t *testing.T) {
	s := newTestStore(t)
	seedTestData(t, s)
	cb := NewContextBuilder(s)

	// Report hint.
	ctx, err := cb.Build("report")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.PatientSummary == "" {
		t.Error("expected non-empty patient summary")
	}
	if len(ctx.RelevantFacts) == 0 {
		t.Error("expected relevant facts for report hint")
	}

	// Medication hint.
	ctx, err = cb.Build("medication")
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.RelevantFacts) == 0 {
		t.Error("expected relevant facts for medication hint")
	}

	// Generic hint.
	ctx, err = cb.Build("chat")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.PatientSummary == "" {
		t.Error("expected patient summary even for generic hint")
	}
}
