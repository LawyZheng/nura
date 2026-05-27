package gateway

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/web"
)

func (gw *Server) handleIndex(c *gin.Context) {
	profiles, err := gw.store.ListPatientProfiles()
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "index.html", map[string]any{
		"Patients": profiles,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (gw *Server) handleDashboard(c *gin.Context) {
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

	diagnoses, _ := gw.store.ListDiagnoses(pid)
	meds, _ := gw.store.ListActiveMedications(pid)
	reports, _ := gw.store.ListRecentReports(pid, 5)
	abnormals, _ := gw.store.GetAbnormalIndicators(pid)
	summaries, _ := gw.store.GetActiveMemorySummaries(pid)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "dashboard.html", map[string]any{
		"Profile":    profile,
		"Diagnoses":  diagnoses,
		"Meds":       meds,
		"Reports":    reports,
		"Abnormals":  abnormals,
		"Summaries":  summaries,
		"PatientID":  pid,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (gw *Server) handleReportList(c *gin.Context) {
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

	reports, _ := gw.store.ListHealthReports(pid)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "reports.html", map[string]any{
		"Profile":   profile,
		"Reports":   reports,
		"PatientID": pid,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

func (gw *Server) handleReportDetail(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid patient id")
		return
	}

	reportID, err := strconv.Atoi(c.Param("report_id"))
	if err != nil {
		c.String(http.StatusBadRequest, "invalid report id")
		return
	}

	report, err := gw.store.GetHealthReport(reportID)
	if err != nil || report.PatientID != pid {
		c.String(http.StatusNotFound, "report not found")
		return
	}

	indicators, _ := gw.store.GetIndicatorsByReport(reportID)

	profile, _ := gw.store.GetPatientProfile(pid)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "report_detail.html", map[string]any{
		"Profile":    profile,
		"Report":     report,
		"Indicators": indicators,
		"PatientID":  pid,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
