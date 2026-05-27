package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/chat"
	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/rag"
	"github.com/LawyZheng/nura/internal/runtime"
	"github.com/LawyZheng/nura/internal/store"
	"github.com/LawyZheng/nura/internal/web"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// Server is the HTTP gateway that exposes the agent API.
type Server struct {
	agent    *runtime.AgentRuntime
	pipeline *pipeline.Pipeline
	traces   *runtime.TraceStore
	store    *store.Store
	chatSvc  *chat.Service
	engine   *gin.Engine
	srv      *http.Server
}

// NewServer creates a new HTTP gateway.
func NewServer(agent *runtime.AgentRuntime, llm providers.LLMProvider, s *store.Store, ks *rag.KnowledgeStore) *Server {
	gw := &Server{
		agent:    agent,
		pipeline: pipeline.NewPipeline(llm, s),
		traces:   agent.Traces(),
		store:    s,
		chatSvc:  chat.NewService(llm, policy.NewEngine(), ks, s),
		engine:   gin.New(),
	}
	gw.engine.Use(gin.Recovery())
	gw.engine.Use(corsMiddleware())
	gw.engine.Use(requestLogger())
	gw.routes()
	return gw
}

func (gw *Server) routes() {
	gw.engine.GET("/health", gw.handleHealth)
	gw.engine.POST("/agent/run", gw.handleAgentRun)
	gw.engine.POST("/agent/report/ingest", gw.handleReportIngest)
	gw.engine.GET("/agent/debug/trace/:trace_id", gw.handleTrace)

	api := gw.engine.Group("/api")
	api.GET("/reports", gw.handleListReports)
	api.GET("/reports/:id", gw.handleGetReport)
	api.GET("/patient/:id", gw.handleGetPatient)
	api.POST("/patient", gw.handleCreatePatient)
	api.PUT("/patient/:id", gw.handleUpdatePatient)
	api.GET("/indicators", gw.handleListIndicators)
	api.POST("/chat", gw.handleChat)
	api.GET("/chat/history", gw.handleChatHistory)
	api.POST("/symptoms", gw.handleCreateSymptom)
	api.GET("/symptoms", gw.handleListSymptoms)
	api.POST("/meals", gw.handleCreateMeal)
	api.GET("/meals", gw.handleListMeals)
	api.POST("/medications", gw.handleCreateMedication)
	api.PUT("/medications/:id", gw.handleUpdateMedication)
	api.GET("/medications", gw.handleListMedications)
	api.POST("/medications/:id/log", gw.handleCreateMedicationLog)
	api.GET("/medications/:id/logs", gw.handleListMedicationLogs)

	// Web UI
	gw.engine.GET("/", gw.handleIndex)
	gw.engine.GET("/patient/:id", gw.handleDashboard)
	gw.engine.GET("/patient/:id/reports", gw.handleReportList)
	gw.engine.GET("/patient/:id/reports/:report_id", gw.handleReportDetail)
	gw.engine.GET("/patient/:id/chat", gw.handleChatPage)
	gw.engine.GET("/patient/:id/symptoms", gw.handleSymptomsPage)
	gw.engine.GET("/patient/:id/meals", gw.handleMealsPage)
	gw.engine.GET("/patient/:id/medications", gw.handleMedicationsPage)
	gw.engine.StaticFS("/static", web.StaticFS())
}

// ListenAndServe starts the HTTP server.
func (gw *Server) ListenAndServe(addr string) error {
	gw.srv = &http.Server{
		Addr:    addr,
		Handler: gw.engine,
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
	return gw.engine
}

// --- middleware ---

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start))
	}
}

// --- handlers ---

// @Summary Health check
// @Description Returns service health status
// @Tags system
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func (gw *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- POST /agent/run ---

type agentRunRequest struct {
	UserMessage string `json:"user_message" binding:"required"`
	TaskHint    string `json:"task_hint,omitempty"`
	PatientID   int    `json:"patient_id" binding:"required"`
}

// @Summary Run agent
// @Description Execute an agent run with the given user message
// @Tags agent
// @Accept json
// @Produce json
// @Param request body agentRunRequest true "Agent run request"
// @Success 200 {object} runtime.RunResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /agent/run [post]
func (gw *Server) handleAgentRun(c *gin.Context) {
	var req agentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_message and patient_id are required"})
		return
	}

	resp, err := gw.agent.Run(c.Request.Context(), runtime.RunRequest{
		UserMessage: req.UserMessage,
		TaskHint:    req.TaskHint,
		PatientID:   req.PatientID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// --- POST /agent/report/ingest ---

type reportIngestRequest struct {
	RawText    string `json:"raw_text" binding:"required"`
	ReportDate string `json:"report_date"`
	SourceType string `json:"source_type,omitempty"`
	PatientID  int    `json:"patient_id" binding:"required"`
}

// @Summary Ingest medical report
// @Description Parse and ingest a raw medical report through the pipeline
// @Tags agent
// @Accept json
// @Produce json
// @Param request body reportIngestRequest true "Report ingest request"
// @Success 200 {object} pipeline.IngestionResult
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /agent/report/ingest [post]
func (gw *Server) handleReportIngest(c *gin.Context) {
	var req reportIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "raw_text and patient_id are required"})
		return
	}

	result, err := gw.pipeline.Run(c.Request.Context(), req.PatientID, req.RawText, req.ReportDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// --- GET /agent/debug/trace/:trace_id ---

// @Summary Get trace
// @Description Retrieve a completed agent trace by ID
// @Tags debug
// @Produce json
// @Param trace_id path string true "Trace ID"
// @Success 200 {object} model.TraceRun
// @Failure 404 {object} map[string]string
// @Router /agent/debug/trace/{trace_id} [get]
func (gw *Server) handleTrace(c *gin.Context) {
	traceID := c.Param("trace_id")

	trace, ok := gw.traces.Get(traceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("trace %s not found", traceID)})
		return
	}

	c.JSON(http.StatusOK, trace)
}
