package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
)

func newTestServer(t *testing.T) (*Server, int) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	pid, err := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	return NewServer(agent, llm, s), pid
}

func TestHealth(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want 'ok'", body["status"])
	}
}

func TestAgentRun(t *testing.T) {
	srv, pid := newTestServer(t)

	body, _ := json.Marshal(agentRunRequest{UserMessage: "什么是 DOB 值", PatientID: pid})
	req := httptest.NewRequest("POST", "/agent/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp runtime.RunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.UserReply == "" {
		t.Error("expected non-empty reply")
	}
	if resp.TraceID == "" {
		t.Error("expected trace ID")
	}
}

func TestAgentRun_MissingFields(t *testing.T) {
	srv, _ := newTestServer(t)

	// Missing both user_message and patient_id.
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/agent/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAgentRun_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/agent/run", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (gin returns 404 for unregistered method+path combos)", w.Code)
	}
}

func TestReportIngest(t *testing.T) {
	srv, pid := newTestServer(t)

	body, _ := json.Marshal(reportIngestRequest{
		RawText:    "test report content",
		ReportDate: "2024-03-01",
		PatientID:  pid,
	})
	req := httptest.NewRequest("POST", "/agent/report/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestReportIngest_MissingFields(t *testing.T) {
	srv, _ := newTestServer(t)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/agent/report/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTrace_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/agent/debug/trace/nonexistent-id", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTrace_AfterRun(t *testing.T) {
	srv, pid := newTestServer(t)

	runBody, _ := json.Marshal(agentRunRequest{UserMessage: "test", PatientID: pid})
	runReq := httptest.NewRequest("POST", "/agent/run", bytes.NewReader(runBody))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runW, runReq)

	var runResp runtime.RunResponse
	json.NewDecoder(runW.Body).Decode(&runResp)

	traceReq := httptest.NewRequest("GET", "/agent/debug/trace/"+runResp.TraceID, nil)
	traceW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(traceW, traceReq)

	if traceW.Code != http.StatusOK {
		t.Errorf("trace status = %d, want 200", traceW.Code)
	}
}

func TestCORS_Preflight(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS origin = %q, want *", got)
	}
}

// --- API endpoint tests ---

func seedTestData(t *testing.T, s *store.Store, pid int) {
	t.Helper()
	// SYNTHETIC DATA - not real patient information
	s.InsertHealthReport(&model.HealthReport{
		PatientID: pid, ReportType: model.ReportGastroscopy,
		ReportDate: "2024-04-20", RawText: "synthetic gastroscopy report",
		IsProcessed: true, AIClassifiedType: "gastroscopy", AISummary: "Synthetic AI summary",
	})
	s.InsertHealthReport(&model.HealthReport{
		PatientID: pid, ReportType: model.ReportHPBreath,
		ReportDate: "2024-04-20", RawText: "synthetic hp breath report",
		IsProcessed: true, AIClassifiedType: "hp_breath",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: 1, Category: "gastroscopy",
		IndicatorName: "HP_status", IndicatorNameCN: "HP状态",
		Value: "阳性", IsAbnormal: true, AbnormalDirection: "positive", MeasuredAt: "2024-04-20",
	})
	s.InsertIndicator(&model.MedicalIndicator{
		PatientID: pid, ReportID: 1, Category: "blood",
		IndicatorName: "WBC", IndicatorNameCN: "白细胞",
		Value: "4.5", Unit: "×10⁹/L", IsAbnormal: false, MeasuredAt: "2024-04-20",
	})
	s.InsertDiagnosis(&model.Diagnosis{
		PatientID: pid, DiagnosisDate: "2024-04-20",
		Condition: "duodenal_ulcer", Detail: "Synthetic diagnosis",
	})
	s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Omeprazole", Category: "ppi",
		Dosage: "20mg", Frequency: "bid", IsActive: true,
	})
}

func newTestServerWithData(t *testing.T) (*Server, *store.Store, int) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	pid, _ := s.CreatePatientProfile(&model.PatientProfile{
		Name: "Test Patient", Gender: "male", BirthDate: "1985-03-15",
		Allergies: model.StringList{"penicillin"},
	})
	seedTestData(t, s, pid)

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	return NewServer(agent, llm, s), s, pid
}

func TestAPI_ListReports(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/reports?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var reports []model.HealthReport
	json.Unmarshal(resp["reports"], &reports)

	if len(reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(reports))
	}
}

func TestAPI_ListReports_MissingPatientID(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/api/reports", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAPI_GetReport(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/reports/1?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)

	if _, ok := resp["report"]; !ok {
		t.Error("missing 'report' field")
	}
	if _, ok := resp["indicators"]; !ok {
		t.Error("missing 'indicators' field")
	}
}

func TestAPI_GetReport_WrongPatient(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/api/reports/1?patient_id=999", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for wrong patient_id", w.Code)
	}
}

func TestAPI_GetPatient(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)

	for _, field := range []string{"profile", "diagnoses", "active_medications", "recent_reports", "abnormal_indicators", "memory_summaries"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing field %q", field)
		}
	}
}

func TestAPI_GetPatient_NotFound(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/api/patient/999", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestAPI_ListIndicators_ByCategory(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/indicators?patient_id=%d&category=gastroscopy", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var indicators []model.MedicalIndicator
	json.Unmarshal(resp["indicators"], &indicators)

	if len(indicators) != 1 {
		t.Errorf("expected 1 gastroscopy indicator, got %d", len(indicators))
	}
}

func TestAPI_ListIndicators_ByName(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/indicators?patient_id=%d&name=WBC", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var indicators []model.MedicalIndicator
	json.Unmarshal(resp["indicators"], &indicators)

	if len(indicators) != 1 {
		t.Errorf("expected 1 WBC indicator, got %d", len(indicators))
	}
}

func TestAPI_CreatePatient(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"name": "Synthetic Patient", "gender": "female", "birth_date": "1990-06-15",
	})
	req := httptest.NewRequest("POST", "/api/patient", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var resp model.PatientProfile
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID <= 0 {
		t.Error("expected positive ID")
	}
	if resp.Name != "Synthetic Patient" {
		t.Errorf("name = %q, want 'Synthetic Patient'", resp.Name)
	}
}

func TestWeb_Index(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "患者列表") {
		t.Error("expected page to contain '患者列表'")
	}
	if !strings.Contains(body, "Test Patient") {
		t.Error("expected page to contain test patient name")
	}
}

func TestWeb_Index_Empty(t *testing.T) {
	dir := t.TempDir()
	s, _ := store.New(filepath.Join(dir, "test.db"))
	defer s.Close()

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := NewServer(agent, llm, s)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "暂无患者记录") {
		t.Error("expected empty state message")
	}
}

func TestAPI_UpdatePatient(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	body, _ := json.Marshal(map[string]any{
		"name": "Updated Name", "allergies": []string{"aspirin"},
	})
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/patient/%d", pid), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp model.PatientProfile
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "Updated Name" {
		t.Errorf("name = %q, want 'Updated Name'", resp.Name)
	}
}
