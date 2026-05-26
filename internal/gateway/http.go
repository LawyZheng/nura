package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
)

// Server is the HTTP gateway that exposes the agent API.
type Server struct {
	agent    *runtime.AgentRuntime
	pipeline *pipeline.Pipeline
	traces   *runtime.TraceStore
	mux      *http.ServeMux
	srv      *http.Server
}

// NewServer creates a new HTTP gateway.
func NewServer(agent *runtime.AgentRuntime, llm providers.LLMProvider, s *store.Store) *Server {
	gw := &Server{
		agent:    agent,
		pipeline: pipeline.NewPipeline(llm, s),
		traces:   agent.Traces(),
		mux:      http.NewServeMux(),
	}
	gw.routes()
	return gw
}

func (gw *Server) routes() {
	gw.mux.HandleFunc("/health", gw.handleHealth)
	gw.mux.HandleFunc("/agent/run", gw.handleAgentRun)
	gw.mux.HandleFunc("/agent/report/ingest", gw.handleReportIngest)
	gw.mux.HandleFunc("/agent/debug/trace/", gw.handleTrace)
}

// ListenAndServe starts the HTTP server.
func (gw *Server) ListenAndServe(addr string) error {
	gw.srv = &http.Server{
		Addr:    addr,
		Handler: gw.mux,
	}
	log.Printf("nura gateway listening on %s", addr)
	return gw.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (gw *Server) Shutdown(ctx context.Context) error {
	if gw.srv != nil {
		return gw.srv.Shutdown(ctx)
	}
	return nil
}

// Handler returns the http.Handler for testing.
func (gw *Server) Handler() http.Handler {
	return gw.mux
}

func (gw *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- POST /agent/run ---

type agentRunRequest struct {
	UserMessage string `json:"user_message"`
	TaskHint    string `json:"task_hint,omitempty"`
	PatientID   string `json:"patient_id,omitempty"`
}

func (gw *Server) handleAgentRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req agentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.UserMessage == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_message is required"})
		return
	}

	resp, err := gw.agent.Run(r.Context(), runtime.RunRequest{
		UserMessage: req.UserMessage,
		TaskHint:    req.TaskHint,
		PatientID:   req.PatientID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- POST /agent/report/ingest ---

type reportIngestRequest struct {
	RawText    string `json:"raw_text"`
	ReportDate string `json:"report_date"`
	SourceType string `json:"source_type,omitempty"`
	PatientID  string `json:"patient_id,omitempty"`
}

func (gw *Server) handleReportIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req reportIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.RawText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "raw_text is required"})
		return
	}

	result, err := gw.pipeline.Run(r.Context(), req.RawText, req.ReportDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// --- GET /agent/debug/trace/{trace_id} ---

func (gw *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Extract trace_id from path: /agent/debug/trace/{trace_id}
	traceID := strings.TrimPrefix(r.URL.Path, "/agent/debug/trace/")
	if traceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trace_id is required"})
		return
	}

	trace, ok := gw.traces.Get(traceID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("trace %s not found", traceID)})
		return
	}

	writeJSON(w, http.StatusOK, trace)
}

// --- helpers ---

// writeJSON is a helper that serializes v as JSON and writes it to w.
// It is only used by this package, so it stays unexported and internal.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Ensure TraceRun satisfies the JSON contract at compile time.
var _ json.Marshaler // not enforced, but documents intent
var _ = model.TraceRun{}
