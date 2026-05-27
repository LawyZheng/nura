package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
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
	agent := runtime.NewAgentRuntime(llm, pe, s)
	srv := NewServer(agent, llm, s)

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
