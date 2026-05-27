package gateway

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/web"
)

type createMealRequest struct {
	PatientID   int      `json:"patient_id" binding:"required"`
	MealType    string   `json:"meal_type" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	IrritantTags []string `json:"irritant_tags"`
}

func (gw *Server) handleCreateMeal(c *gin.Context) {
	var req createMealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "patient_id, meal_type, and content are required"})
		return
	}

	hasIrritant := len(req.IrritantTags) > 0
	irritantDetail := strings.Join(req.IrritantTags, ",")

	ml := &model.MealLog{
		PatientID:      req.PatientID,
		MealType:       req.MealType,
		Content:        req.Content,
		HasIrritant:    hasIrritant,
		IrritantDetail: irritantDetail,
		RecordedAt:     time.Now(),
	}

	if _, err := gw.store.InsertMealLog(ml); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"meal": ml})
}

func (gw *Server) handleListMeals(c *gin.Context) {
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

	meals, err := gw.store.ListMealLogs(pid, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"meals": meals})
}

func (gw *Server) handleMealsPage(c *gin.Context) {
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

	meals, _ := gw.store.ListMealLogs(pid, 20)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "meals.html", map[string]any{
		"Profile":   profile,
		"Meals":     meals,
		"PatientID": pid,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
