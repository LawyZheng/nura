package store

import (
	"fmt"
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

func createTestPatient(t *testing.T, s *Store) int {
	t.Helper()
	id, err := s.CreatePatientProfile(&model.PatientProfile{
		Name:   "Test Patient",
		Gender: "male",
	})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}
	return id
}

func TestStore_PatientProfile(t *testing.T) {
	s := newTestStore(t)

	id, err := s.CreatePatientProfile(&model.PatientProfile{
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
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	p, err := s.GetPatientProfile(id)
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
	p.Name = "Updated Patient"
	p.Weight = 72
	err = s.UpdatePatientProfile(p)
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.GetPatientProfile(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Updated Patient" {
		t.Errorf("updated name = %q, want 'Updated Patient'", p.Name)
	}
}

func TestStore_MultiplePatients(t *testing.T) {
	s := newTestStore(t)

	id1, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Patient A"})
	id2, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Patient B"})

	if id1 == id2 {
		t.Error("expected different IDs for different patients")
	}

	p1, _ := s.GetPatientProfile(id1)
	p2, _ := s.GetPatientProfile(id2)
	if p1.Name != "Patient A" || p2.Name != "Patient B" {
		t.Errorf("names = %q, %q; want 'Patient A', 'Patient B'", p1.Name, p2.Name)
	}
}

func TestStore_HealthReport(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	id, err := s.InsertHealthReport(&model.HealthReport{
		PatientID:  pid,
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
	if r.PatientID != pid {
		t.Errorf("patient_id = %d, want %d", r.PatientID, pid)
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

func TestStore_HealthReport_Isolation(t *testing.T) {
	s := newTestStore(t)
	pid1 := createTestPatient(t, s)
	pid2, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Patient B"})

	s.InsertHealthReport(&model.HealthReport{PatientID: pid1, ReportType: model.ReportGastroscopy, ReportDate: "2024-01-01", RawText: "p1"})
	s.InsertHealthReport(&model.HealthReport{PatientID: pid2, ReportType: model.ReportBloodRoutine, ReportDate: "2024-01-01", RawText: "p2"})

	r1, _ := s.ListHealthReports(pid1)
	r2, _ := s.ListHealthReports(pid2)

	if len(r1) != 1 || len(r2) != 1 {
		t.Errorf("expected 1 report each, got %d and %d", len(r1), len(r2))
	}
}

func TestStore_MedicalIndicator(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	reportID, _ := s.InsertHealthReport(&model.HealthReport{
		PatientID:  pid,
		ReportType: model.ReportBloodRoutine,
		ReportDate: "2024-03-01",
		RawText:    "blood test",
	})

	refLow := 130.0
	refHigh := 175.0
	_, err := s.InsertIndicator(&model.MedicalIndicator{
		PatientID:         pid,
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

	timeline, err := s.GetIndicatorTimeline(pid, "hemoglobin")
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 1 {
		t.Errorf("expected 1 timeline entry, got %d", len(timeline))
	}
}

func TestStore_Diagnosis(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	id, err := s.InsertDiagnosis(&model.Diagnosis{
		PatientID:     pid,
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

	diagnoses, err := s.ListDiagnoses(pid)
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
	pid := createTestPatient(t, s)

	painScore := 5
	_, err := s.InsertSymptomLog(&model.SymptomLog{
		PatientID:    pid,
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

	logs, err := s.ListSymptomLogs(pid, 10)
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
	pid := createTestPatient(t, s)

	_, err := s.InsertMedication(&model.Medication{
		PatientID:   pid,
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

	meds, err := s.ListActiveMedications(pid)
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
	pid := createTestPatient(t, s)

	_, err := s.InsertMemorySummary(&model.MemorySummary{
		PatientID: pid,
		Category:  "symptom_pattern",
		Content:   "Patient reports worsening pain after spicy food",
	})
	if err != nil {
		t.Fatal(err)
	}

	summaries, err := s.GetActiveMemorySummaries(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
}

func TestStore_MealLog(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	_, err := s.InsertMealLog(&model.MealLog{
		PatientID:   pid,
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
	pid := createTestPatient(t, s)

	_, err := s.InsertAIInsight(&model.AIInsight{
		PatientID:   pid,
		InsightType: "trend",
		Content:     "Pain scores improving over last 7 days",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestStore_ListPatientProfiles(t *testing.T) {
	s := newTestStore(t)

	profiles, err := s.ListPatientProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}

	s.CreatePatientProfile(&model.PatientProfile{Name: "Alice"})
	s.CreatePatientProfile(&model.PatientProfile{Name: "Bob"})

	profiles, err = s.ListPatientProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestStore_GetIndicatorsByCategory(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	rid1, _ := s.InsertHealthReport(&model.HealthReport{
		PatientID: pid, ReportType: model.ReportBloodRoutine,
		ReportDate: "2024-03-01", RawText: "blood test",
	})
	rid2, _ := s.InsertHealthReport(&model.HealthReport{
		PatientID: pid, ReportType: model.ReportGastroscopy,
		ReportDate: "2024-03-01", RawText: "gastroscopy",
	})

	// SYNTHETIC DATA - not real patient information
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: rid1, Category: "blood",
		IndicatorName: "WBC", Value: "4.5", Unit: "×10⁹/L", MeasuredAt: "2024-03-01",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: rid1, Category: "blood",
		IndicatorName: "RBC", Value: "4.2", Unit: "×10¹²/L", MeasuredAt: "2024-03-01",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: rid2, Category: "gastroscopy",
		IndicatorName: "HP_status", Value: "阳性", MeasuredAt: "2024-03-01",
	})

	blood, err := s.GetIndicatorsByCategory(pid, "blood")
	if err != nil {
		t.Fatal(err)
	}
	if len(blood) != 2 {
		t.Errorf("expected 2 blood indicators, got %d", len(blood))
	}

	gastro, err := s.GetIndicatorsByCategory(pid, "gastroscopy")
	if err != nil {
		t.Fatal(err)
	}
	if len(gastro) != 1 {
		t.Errorf("expected 1 gastroscopy indicator, got %d", len(gastro))
	}

	empty, err := s.GetIndicatorsByCategory(pid, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 indicators for nonexistent category, got %d", len(empty))
	}
}

func TestStore_GetAbnormalIndicators(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	rid, _ := s.InsertHealthReport(&model.HealthReport{
		PatientID: pid, ReportType: model.ReportBloodRoutine,
		ReportDate: "2024-03-01", RawText: "blood test",
	})

	// SYNTHETIC DATA - not real patient information
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: rid, Category: "blood",
		IndicatorName: "WBC", Value: "3.2", Unit: "×10⁹/L",
		IsAbnormal: true, AbnormalDirection: "low", MeasuredAt: "2024-03-01",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: rid, Category: "blood",
		IndicatorName: "RBC", Value: "4.5", Unit: "×10¹²/L",
		IsAbnormal: false, MeasuredAt: "2024-03-01",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: rid, Category: "blood",
		IndicatorName: "PLT", Value: "380", Unit: "×10⁹/L",
		IsAbnormal: true, AbnormalDirection: "high", MeasuredAt: "2024-03-01",
	})

	abnormals, err := s.GetAbnormalIndicators(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(abnormals) != 2 {
		t.Fatalf("expected 2 abnormal indicators, got %d", len(abnormals))
	}
	for _, ind := range abnormals {
		if !ind.IsAbnormal {
			t.Errorf("indicator %s should be abnormal", ind.IndicatorName)
		}
	}
}

func TestStore_GetAbnormalIndicators_Isolation(t *testing.T) {
	s := newTestStore(t)
	pid1 := createTestPatient(t, s)
	pid2, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Patient B"})

	rid1, _ := s.InsertHealthReport(&model.HealthReport{
		PatientID: pid1, ReportType: model.ReportBloodRoutine,
		ReportDate: "2024-03-01", RawText: "p1 blood",
	})
	rid2, _ := s.InsertHealthReport(&model.HealthReport{
		PatientID: pid2, ReportType: model.ReportBloodRoutine,
		ReportDate: "2024-03-01", RawText: "p2 blood",
	})

	// SYNTHETIC DATA - not real patient information
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid1, ReportID: rid1, Category: "blood",
		IndicatorName: "WBC", Value: "3.2", IsAbnormal: true, MeasuredAt: "2024-03-01",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid2, ReportID: rid2, Category: "blood",
		IndicatorName: "WBC", Value: "2.8", IsAbnormal: true, MeasuredAt: "2024-03-01",
	})

	p1Abnormals, _ := s.GetAbnormalIndicators(pid1)
	p2Abnormals, _ := s.GetAbnormalIndicators(pid2)

	if len(p1Abnormals) != 1 || len(p2Abnormals) != 1 {
		t.Errorf("expected 1 each, got %d and %d", len(p1Abnormals), len(p2Abnormals))
	}
}

func TestStore_ListRecentReports(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	s.InsertHealthReport(&model.HealthReport{PatientID: pid, ReportType: model.ReportBloodRoutine, ReportDate: "2024-01-01", RawText: "r1"})
	s.InsertHealthReport(&model.HealthReport{PatientID: pid, ReportType: model.ReportGastroscopy, ReportDate: "2024-02-01", RawText: "r2"})
	s.InsertHealthReport(&model.HealthReport{PatientID: pid, ReportType: model.ReportHPBreath, ReportDate: "2024-03-01", RawText: "r3"})
	s.InsertHealthReport(&model.HealthReport{PatientID: pid, ReportType: model.ReportLiverFunction, ReportDate: "2024-04-01", RawText: "r4"})

	recent, err := s.ListRecentReports(pid, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent reports, got %d", len(recent))
	}
	if recent[0].ReportDate != "2024-04-01" {
		t.Errorf("first report date = %q, want 2024-04-01", recent[0].ReportDate)
	}
	if recent[1].ReportDate != "2024-03-01" {
		t.Errorf("second report date = %q, want 2024-03-01", recent[1].ReportDate)
	}

	all, err := s.ListRecentReports(pid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Errorf("expected 4 reports with limit 10, got %d", len(all))
	}
}

func TestStore_SupersedeMemorySummary(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	oldID, _ := s.InsertMemorySummary(&model.MemorySummary{
		PatientID: pid, Category: "overall", Content: "old summary",
	})
	newID, _ := s.InsertMemorySummary(&model.MemorySummary{
		PatientID: pid, Category: "overall", Content: "new summary",
	})

	err := s.SupersedeMemorySummary(oldID, newID)
	if err != nil {
		t.Fatal(err)
	}

	active, err := s.GetActiveMemorySummaries(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active summary after supersede, got %d", len(active))
	}
	if active[0].ID != newID {
		t.Errorf("active summary id = %d, want %d", active[0].ID, newID)
	}
}

func TestStore_ChatMessage(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	id, err := s.InsertChatMessage(&model.ChatMessage{
		PatientID: pid,
		Role:      "user",
		Content:   "我现在在吃奥美拉唑，能不能停药？",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	_, err = s.InsertChatMessage(&model.ChatMessage{
		PatientID:  pid,
		Role:       "assistant",
		Content:    "奥美拉唑是质子泵抑制剂...",
		Sources:    model.StringList{"HP根治四联疗法常用药物库"},
		Confidence: "medium",
		RiskLevel:  "high",
	})
	if err != nil {
		t.Fatal(err)
	}

	msgs, err := s.ListRecentChatMessages(pid, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	// Should be in chronological order (ASC).
	if msgs[0].Role != "user" {
		t.Errorf("first message role = %q, want 'user'", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("second message role = %q, want 'assistant'", msgs[1].Role)
	}
	if len(msgs[1].Sources) != 1 || msgs[1].Sources[0] != "HP根治四联疗法常用药物库" {
		t.Errorf("sources = %v, want [HP根治四联疗法常用药物库]", msgs[1].Sources)
	}
}

func TestStore_ChatMessage_Isolation(t *testing.T) {
	s := newTestStore(t)
	pid1 := createTestPatient(t, s)
	pid2, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Patient B"})

	// SYNTHETIC DATA - not real patient information
	s.InsertChatMessage(&model.ChatMessage{PatientID: pid1, Role: "user", Content: "p1 msg"})
	s.InsertChatMessage(&model.ChatMessage{PatientID: pid2, Role: "user", Content: "p2 msg"})

	m1, _ := s.ListRecentChatMessages(pid1, 10)
	m2, _ := s.ListRecentChatMessages(pid2, 10)

	if len(m1) != 1 || len(m2) != 1 {
		t.Errorf("expected 1 each, got %d and %d", len(m1), len(m2))
	}
}

func TestStore_ChatMessage_Limit(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	for i := 0; i < 5; i++ {
		s.InsertChatMessage(&model.ChatMessage{
			PatientID: pid, Role: "user", Content: fmt.Sprintf("msg %d", i),
		})
	}

	msgs, err := s.ListRecentChatMessages(pid, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages with limit, got %d", len(msgs))
	}
	// Should return the 3 most recent in chronological order.
	if msgs[0].Content != "msg 2" {
		t.Errorf("first message = %q, want 'msg 2'", msgs[0].Content)
	}
	if msgs[2].Content != "msg 4" {
		t.Errorf("last message = %q, want 'msg 4'", msgs[2].Content)
	}
}

func TestStore_NewWithInvalidPath(t *testing.T) {
	_, err := New("/dev/null/nonexistent/impossible/test.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// --- Phase 3 Store extension tests ---

func TestStore_ListSymptomLogsByDateRange(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		ps := 3 + i
		s.InsertSymptomLog(&model.SymptomLog{
			PatientID:    pid,
			PainScore:    &ps,
			PainLocation: "upper_abdomen",
			RecordedAt:   base.AddDate(0, 0, i),
		})
	}

	start := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 23, 23, 59, 59, 0, time.UTC)
	logs, err := s.ListSymptomLogsByDateRange(pid, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Errorf("expected 3 logs in range, got %d", len(logs))
	}
}

func TestStore_ListMealLogs(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	for i := 0; i < 5; i++ {
		s.InsertMealLog(&model.MealLog{
			PatientID:  pid,
			MealType:   "lunch",
			Content:    fmt.Sprintf("synthetic meal %d", i),
			RecordedAt: time.Now().Add(time.Duration(i) * time.Hour),
		})
	}

	meals, err := s.ListMealLogs(pid, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(meals) != 3 {
		t.Fatalf("expected 3 meals, got %d", len(meals))
	}
}

func TestStore_ListMealLogsByDateRange(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		s.InsertMealLog(&model.MealLog{
			PatientID:  pid,
			MealType:   "lunch",
			Content:    fmt.Sprintf("synthetic meal %d", i),
			RecordedAt: base.AddDate(0, 0, i),
		})
	}

	start := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 24, 23, 59, 59, 0, time.UTC)
	meals, err := s.ListMealLogsByDateRange(pid, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(meals) != 3 {
		t.Errorf("expected 3 meals in range, got %d", len(meals))
	}
}

func TestStore_GetMedication(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	id, err := s.InsertMedication(&model.Medication{
		PatientID:   pid,
		Name:        "Synthetic Omeprazole",
		Category:    "ppi",
		Dosage:      "20mg",
		Frequency:   "bid",
		CourseStart: "2026-05-20",
		CourseEnd:   "2026-06-02",
		IsActive:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	med, err := s.GetMedication(id)
	if err != nil {
		t.Fatal(err)
	}
	if med.Name != "Synthetic Omeprazole" {
		t.Errorf("name = %q, want 'Synthetic Omeprazole'", med.Name)
	}
}

func TestStore_UpdateMedication(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	id, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	med, _ := s.GetMedication(id)
	med.IsActive = false
	err := s.UpdateMedication(med)
	if err != nil {
		t.Fatal(err)
	}

	updated, _ := s.GetMedication(id)
	if updated.IsActive {
		t.Error("expected is_active=false after update")
	}
}

func TestStore_ListAllMedications(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	s.InsertMedication(&model.Medication{PatientID: pid, Name: "Active Drug", IsActive: true})
	inactiveID, _ := s.InsertMedication(&model.Medication{PatientID: pid, Name: "Inactive Drug", IsActive: true})
	// Deactivate via update (GORM skips zero-value bool on create)
	inactive, _ := s.GetMedication(inactiveID)
	inactive.IsActive = false
	s.UpdateMedication(inactive)

	all, err := s.ListAllMedications(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 medications (active+inactive), got %d", len(all))
	}

	active, _ := s.ListActiveMedications(pid)
	if len(active) != 1 {
		t.Errorf("expected 1 active medication, got %d", len(active))
	}
}

func TestStore_ListMedicationLogs(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	for i := 0; i < 3; i++ {
		s.InsertMedicationLog(&model.MedicationLog{
			MedicationID: medID,
			TakenAt:      time.Now().Add(time.Duration(i) * time.Hour),
			Skipped:      i == 1,
		})
	}

	logs, err := s.ListMedicationLogs(medID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
}

func TestStore_CountMedicationLogsByDateRange(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	base := time.Date(2026, 5, 20, 8, 0, 0, 0, time.UTC)
	s.InsertMedicationLog(&model.MedicationLog{MedicationID: medID, TakenAt: base})
	s.InsertMedicationLog(&model.MedicationLog{MedicationID: medID, TakenAt: base.Add(12 * time.Hour)})
	s.InsertMedicationLog(&model.MedicationLog{MedicationID: medID, TakenAt: base.Add(24 * time.Hour), Skipped: true})
	s.InsertMedicationLog(&model.MedicationLog{MedicationID: medID, TakenAt: base.AddDate(0, 0, 5)}) // out of range

	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 21, 23, 59, 59, 0, time.UTC)
	total, taken, err := s.CountMedicationLogsByDateRange(medID, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if taken != 2 {
		t.Errorf("expected taken=2, got %d", taken)
	}
}

func TestStore_GetLatestAIInsight(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	s.InsertAIInsight(&model.AIInsight{
		PatientID: pid, InsightType: "trend_7d", Content: "old insight",
	})
	s.InsertAIInsight(&model.AIInsight{
		PatientID: pid, InsightType: "trend_7d", Content: "latest insight",
	})
	s.InsertAIInsight(&model.AIInsight{
		PatientID: pid, InsightType: "trend_14d", Content: "different type",
	})

	latest, err := s.GetLatestAIInsight(pid, "trend_7d")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Content != "latest insight" {
		t.Errorf("content = %q, want 'latest insight'", latest.Content)
	}

	_, err = s.GetLatestAIInsight(pid, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent insight type")
	}
}

func TestStore_MedicationReminder(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	scheduled := time.Date(2026, 5, 27, 7, 30, 0, 0, time.UTC)
	id, err := s.InsertMedicationReminder(&model.MedicationReminder{
		MedicationID: medID,
		PatientID:    pid,
		ScheduledAt:  scheduled,
		Label:        "早餐前：Synthetic Drug 20mg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	// Insert another reminder for later
	s.InsertMedicationReminder(&model.MedicationReminder{
		MedicationID: medID,
		PatientID:    pid,
		ScheduledAt:  scheduled.Add(10 * time.Hour),
		Label:        "晚餐前：Synthetic Drug 20mg",
	})

	// List pending before noon — should get only the morning one
	before := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	pending, err := s.ListPendingReminders(pid, before)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending reminder before noon, got %d", len(pending))
	}

	// Mark done
	err = s.MarkReminderDone(id)
	if err != nil {
		t.Fatal(err)
	}

	pending, _ = s.ListPendingReminders(pid, before)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after marking done, got %d", len(pending))
	}
}

func TestStore_DeleteRemindersForMedication(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	// SYNTHETIC DATA - not real patient information
	med1, _ := s.InsertMedication(&model.Medication{PatientID: pid, Name: "Drug A", IsActive: true})
	med2, _ := s.InsertMedication(&model.Medication{PatientID: pid, Name: "Drug B", IsActive: true})

	scheduled := time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC)
	s.InsertMedicationReminder(&model.MedicationReminder{MedicationID: med1, PatientID: pid, ScheduledAt: scheduled, Label: "A"})
	s.InsertMedicationReminder(&model.MedicationReminder{MedicationID: med2, PatientID: pid, ScheduledAt: scheduled, Label: "B"})

	err := s.DeleteRemindersForMedication(med1)
	if err != nil {
		t.Fatal(err)
	}

	before := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	pending, _ := s.ListPendingReminders(pid, before)
	if len(pending) != 1 {
		t.Fatalf("expected 1 remaining reminder, got %d", len(pending))
	}
	if pending[0].Label != "B" {
		t.Errorf("remaining reminder label = %q, want 'B'", pending[0].Label)
	}
}
