package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/web"
)

type createMedicationRequest struct {
	PatientID   int    `json:"patient_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Category    string `json:"category"`
	Dosage      string `json:"dosage"`
	Frequency   string `json:"frequency"`
	TimeOfDay   string `json:"time_of_day"`
	CourseStart string `json:"course_start"`
	CourseEnd   string `json:"course_end"`
}

type updateMedicationRequest struct {
	IsActive  *bool  `json:"is_active"`
	Dosage    string `json:"dosage"`
	Frequency string `json:"frequency"`
	CourseEnd string `json:"course_end"`
}

type createMedicationLogRequest struct {
	Skipped bool   `json:"skipped"`
	Note    string `json:"note"`
}

func (gw *Server) handleCreateMedication(c *gin.Context) {
	var req createMedicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "patient_id and name are required"})
		return
	}

	med := &model.Medication{
		PatientID:   req.PatientID,
		Name:        req.Name,
		Category:    req.Category,
		Dosage:      req.Dosage,
		Frequency:   req.Frequency,
		TimeOfDay:   req.TimeOfDay,
		CourseStart: req.CourseStart,
		CourseEnd:   req.CourseEnd,
		IsActive:    true,
	}

	if _, err := gw.store.InsertMedication(med); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"medication": med})
}

func (gw *Server) handleUpdateMedication(c *gin.Context) {
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

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid medication id"})
		return
	}

	med, err := gw.store.GetMedication(id)
	if err != nil || med.PatientID != pid {
		c.JSON(http.StatusNotFound, gin.H{"error": "medication not found"})
		return
	}

	var req updateMedicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsActive != nil {
		med.IsActive = *req.IsActive
	}
	if req.Dosage != "" {
		med.Dosage = req.Dosage
	}
	if req.Frequency != "" {
		med.Frequency = req.Frequency
	}
	if req.CourseEnd != "" {
		med.CourseEnd = req.CourseEnd
	}

	if err := gw.store.UpdateMedication(med); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"medication": med})
}

func (gw *Server) handleListMedications(c *gin.Context) {
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

	meds, err := gw.store.ListAllMedications(pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"medications": meds})
}

func (gw *Server) handleCreateMedicationLog(c *gin.Context) {
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

	medID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid medication id"})
		return
	}

	med, err := gw.store.GetMedication(medID)
	if err != nil || med.PatientID != pid {
		c.JSON(http.StatusNotFound, gin.H{"error": "medication not found"})
		return
	}

	var req createMedicationLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ml := &model.MedicationLog{
		MedicationID: medID,
		TakenAt:      time.Now(),
		Skipped:      req.Skipped,
		Note:         req.Note,
	}

	if _, err := gw.store.InsertMedicationLog(ml); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"log": ml})
}

func (gw *Server) handleListMedicationLogs(c *gin.Context) {
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

	medID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid medication id"})
		return
	}

	med, err := gw.store.GetMedication(medID)
	if err != nil || med.PatientID != pid {
		c.JSON(http.StatusNotFound, gin.H{"error": "medication not found"})
		return
	}

	logs, err := gw.store.ListMedicationLogs(medID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (gw *Server) handleMedicationsPage(c *gin.Context) {
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

	meds, _ := gw.store.ListAllMedications(pid)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "medications.html", map[string]any{
		"Profile":     profile,
		"Medications": meds,
		"PatientID":   pid,
		"Today":       time.Now().Format("2006-01-02"),
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
