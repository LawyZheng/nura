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
	"time"

	goruntime "runtime"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/rag"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
)

func knowledgeDir() string {
	_, file, _, _ := goruntime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "knowledge")
}

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

	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	return NewServer(agent, llm, s, ks), pid
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
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("CORS origin = %q, want http://localhost:3000", got)
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

	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	return NewServer(agent, llm, s, ks), s, pid
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

	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := NewServer(agent, llm, s, ks)

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

func TestWeb_Dashboard(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "健康档案") {
		t.Error("expected page to contain '健康档案'")
	}
	if !strings.Contains(body, "Test Patient") {
		t.Error("expected page to contain patient name")
	}
	if !strings.Contains(body, "Omeprazole") {
		t.Error("expected page to contain medication")
	}
	if !strings.Contains(body, "duodenal_ulcer") {
		t.Error("expected page to contain diagnosis")
	}
}

func TestWeb_ReportList(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/reports", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "检查报告") {
		t.Error("expected page to contain '检查报告'")
	}
	if !strings.Contains(body, "上传新报告") {
		t.Error("expected upload form")
	}
	if !strings.Contains(body, "胃镜") {
		t.Error("expected gastroscopy report in list")
	}
}

func TestWeb_ReportDetail(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/reports/1", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "报告原文") {
		t.Error("expected page to contain '报告原文'")
	}
	if !strings.Contains(body, "HP状态") || !strings.Contains(body, "阳性") {
		t.Error("expected abnormal indicator with highlighting")
	}
	if !strings.Contains(body, "badge-danger") {
		t.Error("expected danger badge for abnormal indicator")
	}
}

func TestWeb_ReportDetail_WrongPatient(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/patient/999/reports/1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestWeb_Dashboard_NotFound(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/patient/999", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
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

// --- Chat endpoint tests ---

func TestAPI_Chat(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	body, _ := json.Marshal(map[string]any{
		"patient_id": pid,
		"message":    "HP呼气试验是什么",
	})
	req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["reply"]; !ok {
		t.Error("missing 'reply' field")
	}
}

func TestAPI_Chat_MissingFields(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAPI_ChatHistory(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	s.InsertChatMessage(&model.ChatMessage{PatientID: pid, Role: "user", Content: "test question"})
	s.InsertChatMessage(&model.ChatMessage{PatientID: pid, Role: "assistant", Content: "test answer"})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/chat/history?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var msgs []model.ChatMessage
	json.Unmarshal(resp["messages"], &msgs)

	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestAPI_ChatHistory_MissingPatientID(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/api/chat/history", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestWeb_ChatPage(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/chat", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "与知愈对话") {
		t.Error("expected chat page title")
	}
}

// --- T2: Symptom recording tests ---

func TestAPI_CreateSymptom(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id":    pid,
		"pain_score":    6,
		"pain_location": "upper_abdomen",
		"pain_timing":   "fasting",
		"stool_color":   "normal",
		"bloating":      true,
		"note":          "synthetic symptom entry",
	})
	req := httptest.NewRequest("POST", "/api/symptoms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["symptom"]; !ok {
		t.Error("missing 'symptom' field")
	}

	var emergency bool
	json.Unmarshal(resp["emergency"], &emergency)
	if emergency {
		t.Error("expected emergency=false for normal stool")
	}
}

func TestAPI_CreateSymptom_Emergency_BlackStool(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id":  pid,
		"pain_score":  5,
		"stool_color": "black",
	})
	req := httptest.NewRequest("POST", "/api/symptoms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["emergency"] != true {
		t.Error("expected emergency=true for black stool")
	}
	if resp["emergency_message"] == nil || resp["emergency_message"] == "" {
		t.Error("expected emergency_message")
	}
}

func TestAPI_CreateSymptom_Emergency_SeverePain(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id": pid,
		"pain_score": 9,
	})
	req := httptest.NewRequest("POST", "/api/symptoms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["emergency"] != true {
		t.Error("expected emergency=true for pain_score=9")
	}
}

func TestAPI_CreateSymptom_MissingPatientID(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	body, _ := json.Marshal(map[string]any{"pain_score": 5})
	req := httptest.NewRequest("POST", "/api/symptoms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAPI_ListSymptoms(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	ps := 5
	s.InsertSymptomLog(&model.SymptomLog{
		PatientID: pid, PainScore: &ps, PainLocation: "upper_abdomen",
		RecordedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/symptoms?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var symptoms []model.SymptomLog
	json.Unmarshal(resp["symptoms"], &symptoms)

	if len(symptoms) != 1 {
		t.Errorf("expected 1 symptom, got %d", len(symptoms))
	}
}

func TestWeb_SymptomsPage(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/symptoms", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "症状记录") {
		t.Error("expected page title '症状记录'")
	}
}

// --- T3: Diet recording tests ---

func TestAPI_CreateMeal(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id":    pid,
		"meal_type":     "lunch",
		"content":       "synthetic meal: rice and fish",
		"irritant_tags": []string{},
	})
	req := httptest.NewRequest("POST", "/api/meals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["meal"]; !ok {
		t.Error("missing 'meal' field")
	}
}

func TestAPI_CreateMeal_WithIrritants(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id":    pid,
		"meal_type":     "lunch",
		"content":       "synthetic spicy hotpot",
		"irritant_tags": []string{"spicy", "oily"},
	})
	req := httptest.NewRequest("POST", "/api/meals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	meal := resp["meal"].(map[string]any)
	if meal["has_irritant"] != true {
		t.Error("expected has_irritant=true for irritant_tags")
	}
}

func TestAPI_CreateMeal_MissingFields(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	body, _ := json.Marshal(map[string]any{"meal_type": "lunch"})
	req := httptest.NewRequest("POST", "/api/meals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAPI_ListMeals(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	s.InsertMealLog(&model.MealLog{
		PatientID: pid, MealType: "lunch", Content: "synthetic rice",
		RecordedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/meals?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var meals []model.MealLog
	json.Unmarshal(resp["meals"], &meals)

	if len(meals) != 1 {
		t.Errorf("expected 1 meal, got %d", len(meals))
	}
}

func TestWeb_MealsPage(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/meals", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "饮食记录") {
		t.Error("expected page title '饮食记录'")
	}
}

// --- T4: Medication tracking tests ---

func TestAPI_CreateMedication(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id":   pid,
		"name":         "Synthetic Amoxicillin",
		"category":     "antibiotic",
		"dosage":       "1g",
		"frequency":    "bid",
		"time_of_day":  "早晚餐后",
		"course_start": "2026-05-20",
		"course_end":   "2026-06-02",
	})
	req := httptest.NewRequest("POST", "/api/medications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["medication"]; !ok {
		t.Error("missing 'medication' field")
	}
}

func TestAPI_UpdateMedication(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug X", IsActive: true,
	})

	body, _ := json.Marshal(map[string]any{"is_active": false})
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/medications/%d?patient_id=%d", medID, pid), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

func TestAPI_ListMedications(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/medications?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var meds []model.Medication
	json.Unmarshal(resp["medications"], &meds)

	if len(meds) == 0 {
		t.Error("expected at least 1 medication from seed data")
	}
}

func TestAPI_CreateMedicationLog(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	body, _ := json.Marshal(map[string]any{"skipped": false, "note": "taken on time"})
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/medications/%d/log?patient_id=%d", medID, pid), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
}

func TestAPI_ListMedicationLogs(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})
	s.InsertMedicationLog(&model.MedicationLog{MedicationID: medID, TakenAt: time.Now()})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/medications/%d/logs?patient_id=%d", medID, pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var logs []model.MedicationLog
	json.Unmarshal(resp["logs"], &logs)

	if len(logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(logs))
	}
}

func TestWeb_MedicationsPage(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/medications", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "用药管理") {
		t.Error("expected page title '用药管理'")
	}
}

// --- T5: Trend visualization tests ---

func TestAPI_GetTrends(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	ps := 6
	s.InsertSymptomLog(&model.SymptomLog{
		PatientID: pid, PainScore: &ps, PainLocation: "upper_abdomen",
		RecordedAt: time.Now(),
	})
	s.InsertMealLog(&model.MealLog{
		PatientID: pid, MealType: "lunch", Content: "synthetic spicy food",
		HasIrritant: true, IrritantDetail: "spicy",
		RecordedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/trends?patient_id=%d&days=7", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	for _, field := range []string{"symptoms", "meals", "medications", "period"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing field %q", field)
		}
	}
}

func TestAPI_GetTrends_MissingPatientID(t *testing.T) {
	srv, _, _ := newTestServerWithData(t)

	req := httptest.NewRequest("GET", "/api/trends", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAPI_GenerateInsight(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	body, _ := json.Marshal(map[string]any{
		"patient_id": pid,
		"days":       7,
	})
	req := httptest.NewRequest("POST", "/api/trends/insight", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["insight"]; !ok {
		t.Error("missing 'insight' field")
	}
	if _, ok := resp["disclaimer"]; !ok {
		t.Error("missing 'disclaimer' field")
	}
}

// --- T7: Reminder tests ---

func TestAPI_PendingReminders(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})
	s.InsertMedicationReminder(&model.MedicationReminder{
		MedicationID: medID, PatientID: pid,
		ScheduledAt: time.Now().Add(-1 * time.Hour),
		Label:       "test reminder",
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/reminders/pending?patient_id=%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	var reminders []map[string]any
	json.Unmarshal(resp["reminders"], &reminders)
	if len(reminders) != 1 {
		t.Errorf("expected 1 pending reminder, got %d", len(reminders))
	}
}

func TestAPI_MarkReminderDone(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})
	remID, _ := s.InsertMedicationReminder(&model.MedicationReminder{
		MedicationID: medID, PatientID: pid,
		ScheduledAt: time.Now().Add(-1 * time.Hour),
		Label:       "test reminder",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/reminders/%d/done?patient_id=%d", remID, pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestWeb_TrendsPage(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/trends", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "趋势分析") {
		t.Error("expected page title '趋势分析'")
	}
	if !strings.Contains(body, "/static/chart.umd.min.js") || !strings.Contains(body, "Chart") {
		t.Error("expected local Chart.js reference")
	}
}
