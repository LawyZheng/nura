package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/web"
)

func (gw *Server) handleGetTrends(c *gin.Context) {
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

	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 7
	}

	end := time.Now()
	start := end.AddDate(0, 0, -days)

	symptoms, _ := gw.store.ListSymptomLogsByDateRange(pid, start, end)
	meals, _ := gw.store.ListMealLogsByDateRange(pid, start, end)
	medications, _ := gw.store.ListActiveMedications(pid)

	c.JSON(http.StatusOK, gin.H{
		"symptoms":    symptoms,
		"meals":       meals,
		"medications": medications,
		"period": gin.H{
			"start": start.Format("2006-01-02"),
			"end":   end.Format("2006-01-02"),
		},
	})
}

func (gw *Server) handleTrendsPage(c *gin.Context) {
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

	latest, _ := gw.store.GetLatestAIInsight(pid, "trend_7d")

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "trends.html", map[string]any{
		"Profile":   profile,
		"PatientID": pid,
		"Insight":   latest,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
