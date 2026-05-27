package gateway

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/web"
)

type createSymptomRequest struct {
	PatientID    int    `json:"patient_id" binding:"required"`
	PainScore    *int   `json:"pain_score"`
	PainLocation string `json:"pain_location"`
	PainTiming   string `json:"pain_timing"`
	StoolColor   string `json:"stool_color"`
	Bloating     bool   `json:"bloating"`
	Nausea       bool   `json:"nausea"`
	AcidReflux   bool   `json:"acid_reflux"`
	Note         string `json:"note"`
}

func isEmergencySymptom(sl *model.SymptomLog) (bool, string) {
	if sl.StoolColor == "black" || sl.StoolColor == "bloody" {
		return true, "检测到黑便/血便记录。请立即就医或拨打 120 急救电话。"
	}
	if sl.PainScore != nil && *sl.PainScore >= 9 {
		return true, "检测到剧烈腹痛（评分≥9）。请立即就医或拨打 120 急救电话。"
	}
	for _, kw := range policy.EmergencyKeywords() {
		if strings.Contains(sl.Note, kw) {
			return true, "检测到紧急健康信号。请立即就医或拨打 120 急救电话。"
		}
	}
	return false, ""
}

func (gw *Server) handleCreateSymptom(c *gin.Context) {
	var req createSymptomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "patient_id is required"})
		return
	}

	sl := &model.SymptomLog{
		PatientID:    req.PatientID,
		PainScore:    req.PainScore,
		PainLocation: req.PainLocation,
		PainTiming:   req.PainTiming,
		StoolColor:   req.StoolColor,
		Bloating:     req.Bloating,
		Nausea:       req.Nausea,
		AcidReflux:   req.AcidReflux,
		Note:         req.Note,
		RecordedAt:   time.Now(),
	}

	if _, err := gw.store.InsertSymptomLog(sl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	emergency, msg := isEmergencySymptom(sl)
	resp := gin.H{"symptom": sl, "emergency": emergency}
	if emergency {
		resp["emergency_message"] = msg
	}
	c.JSON(http.StatusCreated, resp)
}

func (gw *Server) handleListSymptoms(c *gin.Context) {
	pidStr := c.Query("patient_id")
	if pidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "patient_id is required"})
		return
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient_id"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	symptoms, err := gw.store.ListSymptomLogs(pid, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"symptoms": symptoms})
}

func (gw *Server) handleSymptomsPage(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid patient id")
		return
	}

	profile, err := gw.store.GetPatientProfile(pid)
	if err != nil {
		c.String(http.StatusNotFound, "patient not found")
		return
	}

	symptoms, _ := gw.store.ListSymptomLogs(pid, 20)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "symptoms.html", map[string]any{
		"Profile":   profile,
		"Symptoms":  symptoms,
		"PatientID": pid,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
