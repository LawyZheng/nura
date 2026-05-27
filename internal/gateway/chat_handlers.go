package gateway

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/web"
)

type chatRequest struct {
	PatientID int    `json:"patient_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

func (gw *Server) handleChat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "patient_id and message are required"})
		return
	}

	reply, err := gw.chatSvc.HandleMessage(c.Request.Context(), req.PatientID, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reply": reply})
}

func (gw *Server) handleChatHistory(c *gin.Context) {
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

	msgs, err := gw.store.ListRecentChatMessages(pid, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}

func (gw *Server) handleChatPage(c *gin.Context) {
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

	messages, _ := gw.store.ListRecentChatMessages(pid, 20)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := web.Render(c.Writer, "chat.html", map[string]any{
		"Profile":   profile,
		"Messages":  messages,
		"PatientID": pid,
	}); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}
