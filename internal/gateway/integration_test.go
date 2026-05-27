package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LawyZheng/nura/internal/chat"
	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/rag"
	"github.com/LawyZheng/nura/internal/reminder"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
	"github.com/LawyZheng/nura/internal/trend"
	"time"
)

// TestIntegration_FullFlow exercises the complete Phase 2a flow:
// create patient → ingest report → verify API 360 → verify web pages → verify memory update.
func TestIntegration_FullFlow(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	llm := &providers.MockProvider{
		CompleteFunc: func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			if strings.Contains(req.SystemPrompt, "classifier") {
				return &providers.CompletionResponse{Content: "gastroscopy"}, nil
			}
			if strings.Contains(req.SystemPrompt, "extraction") {
				facts := map[string]any{
					"indicators": []map[string]any{
						{"name": "hp_status", "name_cn": "HP状态", "value": "阳性", "is_abnormal": true},
						{"name": "ulcer_stage", "name_cn": "溃疡分期", "value": "A2", "is_abnormal": true},
					},
				}
				data, _ := json.Marshal(facts)
				return &providers.CompletionResponse{Content: string(data)}, nil
			}
			// SYNTHETIC DATA - not real patient information
			return &providers.CompletionResponse{
				Content: "合成报告解读：该胃镜报告显示十二指肠溃疡（A2期）。以上内容仅供健康参考，不构成医疗建议。",
			}, nil
		},
	}
	pe := policy.NewEngine()
	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}
	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := NewServer(agent, llm, s, ks)

	do := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if body != nil {
			req = httptest.NewRequest(method, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		return w
	}

	// Step 1: Create patient via API
	// SYNTHETIC DATA - not real patient information
	createBody, _ := json.Marshal(map[string]any{
		"name": "Synthetic Test Patient", "gender": "male",
		"birth_date": "1985-03-15", "allergies": []string{"penicillin"},
	})
	w := do("POST", "/api/patient", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create patient: status = %d, body = %s", w.Code, w.Body.String())
	}
	var patient model.PatientProfile
	json.NewDecoder(w.Body).Decode(&patient)
	pid := strconv.Itoa(patient.ID)

	// Step 2: Verify patient on homepage
	w = do("GET", "/", nil)
	if !strings.Contains(w.Body.String(), "Synthetic Test Patient") {
		t.Error("patient should appear on homepage")
	}

	// Step 3: Ingest report
	// SYNTHETIC DATA - not real patient information
	ingestBody, _ := json.Marshal(reportIngestRequest{
		RawText:    "合成胃镜检查报告：十二指肠球部溃疡 A2 期。HP 阳性。",
		ReportDate: "2024-04-20",
		PatientID:  patient.ID,
	})
	w = do("POST", "/agent/report/ingest", ingestBody)
	if w.Code != http.StatusOK {
		t.Fatalf("ingest: status = %d, body = %s", w.Code, w.Body.String())
	}
	var ingestResult pipeline.IngestionResult
	json.NewDecoder(w.Body).Decode(&ingestResult)
	if ingestResult.ReportType == "" {
		t.Error("expected non-empty report type")
	}

	// Verify memory update stage ran
	foundMemory := false
	for _, stage := range ingestResult.Stages {
		if stage.StageName == "update_memory" && stage.Error == "" {
			foundMemory = true
		}
	}
	if !foundMemory {
		t.Error("expected successful update_memory stage")
	}

	// Step 4: Verify 360 API
	w = do("GET", "/api/patient/"+pid, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("360 API: status = %d", w.Code)
	}
	var p360 patient360Response
	json.NewDecoder(w.Body).Decode(&p360)
	if p360.Profile == nil {
		t.Fatal("expected profile in 360")
	}
	if len(p360.RecentReports) == 0 {
		t.Error("expected recent reports")
	}
	if len(p360.MemorySummaries) == 0 {
		t.Error("expected memory summaries after ingestion")
	}
	if len(p360.AbnormalIndicators) == 0 {
		t.Error("expected abnormal indicators")
	}

	// Step 5: Verify reports API
	w = do("GET", "/api/reports?patient_id="+pid, nil)
	var reportsResp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&reportsResp)
	var reports []model.HealthReport
	json.Unmarshal(reportsResp["reports"], &reports)
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	rid := strconv.Itoa(reports[0].ID)

	// Step 6: Verify report detail API with indicators
	w = do("GET", "/api/reports/"+rid+"?patient_id="+pid, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("report detail API: status = %d", w.Code)
	}

	// Step 7: Verify web pages render
	w = do("GET", "/patient/"+pid, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "健康档案") {
		t.Error("dashboard should render")
	}
	w = do("GET", "/patient/"+pid+"/reports", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "检查报告") {
		t.Error("report list should render")
	}
	w = do("GET", "/patient/"+pid+"/reports/"+rid, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "报告原文") {
		t.Error("report detail should render")
	}

	// Step 8: Cross-patient isolation
	w = do("GET", "/api/reports/"+rid+"?patient_id=999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-patient access: status = %d, want 404", w.Code)
	}
}

// TestIntegration_ChatFlow exercises the Phase 2b chat flow:
// create patient → send low-risk message → send high-risk message → send emergency →
// verify history persistence → verify chat page renders.
func TestIntegration_ChatFlow(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	llm := &providers.MockProvider{
		CompleteFunc: func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			// SYNTHETIC DATA - not real patient information
			return &providers.CompletionResponse{
				Content:    "合成回复：这是关于您提问的回答。",
				StopReason: "end_turn",
			}, nil
		},
	}
	pe := policy.NewEngine()
	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}
	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := NewServer(agent, llm, s, ks)

	do := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if body != nil {
			req = httptest.NewRequest(method, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		return w
	}

	// Step 1: Create patient
	// SYNTHETIC DATA - not real patient information
	createBody, _ := json.Marshal(map[string]any{
		"name": "Synthetic Chat Patient", "gender": "male",
	})
	w := do("POST", "/api/patient", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create patient: status = %d", w.Code)
	}
	var patient model.PatientProfile
	json.NewDecoder(w.Body).Decode(&patient)
	pid := patient.ID

	// Step 2: Send low-risk message
	chatBody, _ := json.Marshal(map[string]any{
		"patient_id": pid,
		"message":    "HP呼气试验是什么意思",
	})
	w = do("POST", "/api/chat", chatBody)
	if w.Code != http.StatusOK {
		t.Fatalf("low-risk chat: status = %d, body = %s", w.Code, w.Body.String())
	}
	var chatResp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&chatResp)
	var reply chat.ChatReply
	json.Unmarshal(chatResp["reply"], &reply)
	if reply.RiskLevel != "low" {
		t.Errorf("expected low risk, got %q", reply.RiskLevel)
	}
	if reply.Disclaimer == "" {
		t.Error("expected disclaimer on low-risk reply")
	}
	if len(reply.Sources) != 0 {
		t.Errorf("expected no sources on low-risk, got %v", reply.Sources)
	}

	// Step 3: Send high-risk message (medication)
	chatBody, _ = json.Marshal(map[string]any{
		"patient_id": pid,
		"message":    "奥美拉唑能不能停药",
	})
	w = do("POST", "/api/chat", chatBody)
	if w.Code != http.StatusOK {
		t.Fatalf("high-risk chat: status = %d", w.Code)
	}
	json.NewDecoder(w.Body).Decode(&chatResp)
	json.Unmarshal(chatResp["reply"], &reply)
	if reply.RiskLevel != "high" {
		t.Errorf("expected high risk, got %q", reply.RiskLevel)
	}
	if len(reply.Sources) == 0 {
		t.Error("expected RAG sources on high-risk reply")
	}

	// Step 4: Send emergency message
	chatBody, _ = json.Marshal(map[string]any{
		"patient_id": pid,
		"message":    "我吐血了",
	})
	w = do("POST", "/api/chat", chatBody)
	if w.Code != http.StatusOK {
		t.Fatalf("emergency chat: status = %d", w.Code)
	}
	json.NewDecoder(w.Body).Decode(&chatResp)
	json.Unmarshal(chatResp["reply"], &reply)
	if reply.RiskLevel != "emergency" {
		t.Errorf("expected emergency risk, got %q", reply.RiskLevel)
	}
	if !strings.Contains(reply.Content, "立即就医") && !strings.Contains(reply.Content, "急救") {
		t.Error("expected emergency guidance")
	}

	// Step 5: Verify chat history persistence
	w = do("GET", "/api/chat/history?patient_id="+strconv.Itoa(pid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("chat history: status = %d", w.Code)
	}
	var histResp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&histResp)
	var msgs []model.ChatMessage
	json.Unmarshal(histResp["messages"], &msgs)
	// 3 user messages + 3 assistant replies = 6
	if len(msgs) != 6 {
		t.Errorf("expected 6 persisted messages, got %d", len(msgs))
	}

	// Step 6: Verify chat page renders
	w = do("GET", "/patient/"+strconv.Itoa(pid)+"/chat", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("chat page: status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "与知愈对话") {
		t.Error("expected chat page title")
	}

	// Step 7: Cross-patient chat isolation
	// SYNTHETIC DATA - not real patient information
	createBody2, _ := json.Marshal(map[string]any{"name": "Other Patient"})
	w = do("POST", "/api/patient", createBody2)
	var patient2 model.PatientProfile
	json.NewDecoder(w.Body).Decode(&patient2)
	w = do("GET", "/api/chat/history?patient_id="+strconv.Itoa(patient2.ID), nil)
	json.NewDecoder(w.Body).Decode(&histResp)
	json.Unmarshal(histResp["messages"], &msgs)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for other patient, got %d", len(msgs))
	}
}

// TestIntegration_Phase3Flow exercises the Phase 3 daily logging flow:
// create patient → record symptoms → record meals → add medication →
// log dose → query trends → generate AI insight → create/query reminders →
// verify emergency detection.
func TestIntegration_Phase3Flow(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer s.Close()

	llm := &providers.MockProvider{
		CompleteFunc: func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
			// SYNTHETIC DATA - not real patient information
			return &providers.CompletionResponse{
				Content:    "合成分析：近期症状呈好转趋势，建议继续按医嘱用药。",
				StopReason: "end_turn",
			}, nil
		},
	}
	pe := policy.NewEngine()
	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}
	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := NewServer(agent, llm, s, ks)

	do := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if body != nil {
			req = httptest.NewRequest(method, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		return w
	}

	// Step 1: Create patient
	// SYNTHETIC DATA - not real patient information
	createBody, _ := json.Marshal(map[string]any{
		"name": "Synthetic Phase3 Patient", "gender": "male",
	})
	w := do("POST", "/api/patient", createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create patient: status = %d", w.Code)
	}
	var patient model.PatientProfile
	json.NewDecoder(w.Body).Decode(&patient)
	pid := patient.ID
	pidStr := strconv.Itoa(pid)

	// Step 2: Record symptom (normal)
	symptomBody, _ := json.Marshal(map[string]any{
		"patient_id": pid, "pain_score": 5, "pain_location": "upper_abdomen",
		"stool_color": "normal", "bloating": true,
	})
	w = do("POST", "/api/symptoms", symptomBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create symptom: status = %d, body = %s", w.Code, w.Body.String())
	}
	var symptomResp map[string]any
	json.NewDecoder(w.Body).Decode(&symptomResp)
	if symptomResp["emergency"] == true {
		t.Error("expected no emergency for normal symptom")
	}

	// Step 3: Record symptom (emergency — black stool)
	emergencyBody, _ := json.Marshal(map[string]any{
		"patient_id": pid, "pain_score": 7, "stool_color": "black",
	})
	w = do("POST", "/api/symptoms", emergencyBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create emergency symptom: status = %d", w.Code)
	}
	json.NewDecoder(w.Body).Decode(&symptomResp)
	if symptomResp["emergency"] != true {
		t.Error("expected emergency=true for black stool")
	}

	// Step 4: List symptoms
	w = do("GET", "/api/symptoms?patient_id="+pidStr, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list symptoms: status = %d", w.Code)
	}

	// Step 5: Record meal with irritants
	mealBody, _ := json.Marshal(map[string]any{
		"patient_id": pid, "meal_type": "lunch",
		"content": "synthetic spicy hotpot", "irritant_tags": []string{"spicy", "oily"},
	})
	w = do("POST", "/api/meals", mealBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create meal: status = %d", w.Code)
	}
	var mealResp map[string]any
	json.NewDecoder(w.Body).Decode(&mealResp)
	meal := mealResp["meal"].(map[string]any)
	if meal["has_irritant"] != true {
		t.Error("expected has_irritant=true")
	}

	// Step 6: Add medication
	medBody, _ := json.Marshal(map[string]any{
		"patient_id": pid, "name": "Synthetic Omeprazole",
		"category": "ppi", "dosage": "20mg", "frequency": "bid",
		"time_of_day": "早晚餐前",
		"course_start": "2026-05-20", "course_end": "2026-06-02",
	})
	w = do("POST", "/api/medications", medBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("create medication: status = %d, body = %s", w.Code, w.Body.String())
	}
	var medResp map[string]any
	json.NewDecoder(w.Body).Decode(&medResp)
	medData := medResp["medication"].(map[string]any)
	medID := int(medData["id"].(float64))

	// Step 7: Log a dose
	logBody, _ := json.Marshal(map[string]any{"skipped": false, "note": "on time"})
	w = do("POST", fmt.Sprintf("/api/medications/%d/log", medID), logBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("log dose: status = %d", w.Code)
	}

	// Step 8: Query trends
	w = do("GET", "/api/trends?patient_id="+pidStr+"&days=7", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get trends: status = %d", w.Code)
	}
	var trendsResp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&trendsResp)
	for _, field := range []string{"symptoms", "meals", "medications", "period"} {
		if _, ok := trendsResp[field]; !ok {
			t.Errorf("trends missing field %q", field)
		}
	}

	// Step 9: Generate AI insight
	insightBody, _ := json.Marshal(map[string]any{
		"patient_id": pid, "days": 7,
	})
	w = do("POST", "/api/trends/insight", insightBody)
	if w.Code != http.StatusOK {
		t.Fatalf("generate insight: status = %d, body = %s", w.Code, w.Body.String())
	}
	var insightResp trend.InsightResult
	json.NewDecoder(w.Body).Decode(&insightResp)
	if insightResp.Insight == nil {
		t.Fatal("expected non-nil insight")
	}
	if insightResp.Disclaimer == "" {
		t.Error("expected disclaimer on insight")
	}

	// Step 10: Generate reminders
	scheduler := reminder.NewScheduler(s)
	today := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	count, err := scheduler.GenerateDailyReminders(pid, today)
	if err != nil {
		t.Fatalf("generate reminders: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 reminders for bid, got %d", count)
	}

	// Step 11: Query pending reminders
	w = do("GET", fmt.Sprintf("/api/reminders/pending?patient_id=%d", pid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pending reminders: status = %d", w.Code)
	}

	// Step 12: Verify all web pages render
	pages := []struct {
		path  string
		check string
	}{
		{"/patient/" + pidStr, "健康档案"},
		{"/patient/" + pidStr + "/symptoms", "症状记录"},
		{"/patient/" + pidStr + "/meals", "饮食记录"},
		{"/patient/" + pidStr + "/medications", "用药管理"},
		{"/patient/" + pidStr + "/trends", "趋势分析"},
		{"/patient/" + pidStr + "/chat", "与知愈对话"},
	}
	for _, p := range pages {
		w = do("GET", p.path, nil)
		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", p.path, w.Code)
			continue
		}
		if !strings.Contains(w.Body.String(), p.check) {
			t.Errorf("%s: missing %q", p.path, p.check)
		}
	}

	// Step 13: Dashboard nav-links include all Phase 3 pages
	w = do("GET", "/patient/"+pidStr, nil)
	body := w.Body.String()
	for _, link := range []string{"symptoms", "meals", "medications", "trends", "chat"} {
		if !strings.Contains(body, "/patient/"+pidStr+"/"+link) {
			t.Errorf("dashboard missing nav link to %s", link)
		}
	}
}
