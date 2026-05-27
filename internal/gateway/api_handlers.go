package gateway

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/model"
)

func (gw *Server) handleListReports(c *gin.Context) {
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

	reports, err := gw.store.ListHealthReports(pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

func (gw *Server) handleGetReport(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
		return
	}
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

	report, err := gw.store.GetHealthReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}
	if report.PatientID != pid {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}

	indicators, err := gw.store.GetIndicatorsByReport(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"report": report, "indicators": indicators})
}

type patient360Response struct {
	Profile            *model.PatientProfile     `json:"profile"`
	Diagnoses          []*model.Diagnosis        `json:"diagnoses"`
	ActiveMedications  []*model.Medication       `json:"active_medications"`
	RecentReports      []*model.HealthReport     `json:"recent_reports"`
	AbnormalIndicators []*model.MedicalIndicator `json:"abnormal_indicators"`
	MemorySummaries    []*model.MemorySummary    `json:"memory_summaries"`
}

func (gw *Server) handleGetPatient(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient id"})
		return
	}

	profile, err := gw.store.GetPatientProfile(pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "patient not found"})
		return
	}

	diagnoses, _ := gw.store.ListDiagnoses(pid)
	meds, _ := gw.store.ListActiveMedications(pid)
	reports, _ := gw.store.ListRecentReports(pid, 5)
	abnormals, _ := gw.store.GetAbnormalIndicators(pid)
	summaries, _ := gw.store.GetActiveMemorySummaries(pid)

	c.JSON(http.StatusOK, patient360Response{
		Profile:            profile,
		Diagnoses:          diagnoses,
		ActiveMedications:  meds,
		RecentReports:      reports,
		AbnormalIndicators: abnormals,
		MemorySummaries:    summaries,
	})
}

func (gw *Server) handleListIndicators(c *gin.Context) {
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

	category := c.Query("category")
	name := c.Query("name")

	var indicators []*model.MedicalIndicator

	switch {
	case name != "":
		indicators, err = gw.store.GetIndicatorTimeline(pid, name)
	case category != "":
		indicators, err = gw.store.GetIndicatorsByCategory(pid, category)
	default:
		indicators, err = gw.store.GetAbnormalIndicators(pid)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"indicators": indicators})
}

type createPatientRequest struct {
	Name         string           `json:"name"`
	Gender       string           `json:"gender"`
	BirthDate    string           `json:"birth_date"`
	Height       float64          `json:"height"`
	Weight       float64          `json:"weight"`
	Allergies    model.StringList `json:"allergies"`
	MedicalNotes string           `json:"medical_notes"`
}

func (gw *Server) handleCreatePatient(c *gin.Context) {
	var req createPatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile := &model.PatientProfile{
		Name:         req.Name,
		Gender:       req.Gender,
		BirthDate:    req.BirthDate,
		Height:       req.Height,
		Weight:       req.Weight,
		Allergies:    req.Allergies,
		MedicalNotes: req.MedicalNotes,
	}

	id, err := gw.store.CreatePatientProfile(profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	profile.ID = id

	c.JSON(http.StatusCreated, profile)
}

func (gw *Server) handleUpdatePatient(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient id"})
		return
	}

	existing, err := gw.store.GetPatientProfile(pid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "patient not found"})
		return
	}

	var req createPatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Gender != "" {
		existing.Gender = req.Gender
	}
	if req.BirthDate != "" {
		existing.BirthDate = req.BirthDate
	}
	if req.Height > 0 {
		existing.Height = req.Height
	}
	if req.Weight > 0 {
		existing.Weight = req.Weight
	}
	if req.Allergies != nil {
		existing.Allergies = req.Allergies
	}
	if req.MedicalNotes != "" {
		existing.MedicalNotes = req.MedicalNotes
	}

	if err := gw.store.UpdatePatientProfile(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, existing)
}
