package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	llm := &providers.MockProvider{}
	pe := policy.NewEngine()
	agent := runtime.NewAgentRuntime(llm, pe, s)
	return NewServer(agent, llm, s)
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
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
	srv := newTestServer(t)

	body, _ := json.Marshal(agentRunRequest{UserMessage: "什么是 DOB 值"})
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

func TestAgentRun_EmptyMessage(t *testing.T) {
	srv := newTestServer(t)

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
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/agent/run", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (gin returns 404 for unregistered method+path combos)", w.Code)
	}
}

func TestReportIngest(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(reportIngestRequest{
		RawText:    "test report content",
		ReportDate: "2024-03-01",
	})
	req := httptest.NewRequest("POST", "/agent/report/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestReportIngest_EmptyText(t *testing.T) {
	srv := newTestServer(t)

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
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/agent/debug/trace/nonexistent-id", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTrace_AfterRun(t *testing.T) {
	srv := newTestServer(t)

	// First, do an agent run to create a trace.
	runBody, _ := json.Marshal(agentRunRequest{UserMessage: "test"})
	runReq := httptest.NewRequest("POST", "/agent/run", bytes.NewReader(runBody))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runW, runReq)

	var runResp runtime.RunResponse
	json.NewDecoder(runW.Body).Decode(&runResp)

	// Then retrieve the trace.
	traceReq := httptest.NewRequest("GET", "/agent/debug/trace/"+runResp.TraceID, nil)
	traceW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(traceW, traceReq)

	if traceW.Code != http.StatusOK {
		t.Errorf("trace status = %d, want 200", traceW.Code)
	}
}

func TestCORS_Preflight(t *testing.T) {
	srv := newTestServer(t)

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
