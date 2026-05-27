package gateway

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/web"
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

type createShareRequest struct {
	Passcode string `json:"passcode"`
}

func (gw *Server) handleCreateShareLink(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient id"})
		return
	}

	if _, err := gw.store.GetPatientProfile(pid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "patient not found"})
		return
	}

	var req createShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = createShareRequest{}
	}

	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token"})
		return
	}

	link := &model.ShareLink{
		PatientID: pid,
		Token:     token,
		Passcode:  req.Passcode,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		IsActive:  true,
	}
	if err := gw.store.CreateShareLink(link); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create share link"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         link.ID,
		"token":      link.Token,
		"url":        "/share/" + link.Token,
		"expires_at": link.ExpiresAt.Format(time.RFC3339),
	})
}

func (gw *Server) handleListShareLinks(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient id"})
		return
	}

	links, err := gw.store.ListShareLinks(pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list share links"})
		return
	}

	c.JSON(http.StatusOK, links)
}

func (gw *Server) handleDeleteShareLink(c *gin.Context) {
	shareID, err := strconv.Atoi(c.Param("share_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share id"})
		return
	}

	if err := gw.store.DeactivateShareLink(shareID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "deactivate share link"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (gw *Server) handleShareView(c *gin.Context) {
	token := c.Param("token")
	link, err := gw.store.GetShareLinkByToken(token)
	if err != nil || !link.IsActive || time.Now().After(link.ExpiresAt) {
		c.Status(http.StatusNotFound)
		return
	}

	if link.Passcode != "" {
		web.Render(c.Writer, "share_auth.html", map[string]any{
			"Token": token,
			"Error": "",
		})
		return
	}

	gw.renderShareView(c, link.PatientID)
}

func (gw *Server) handleShareVerify(c *gin.Context) {
	token := c.Param("token")
	link, err := gw.store.GetShareLinkByToken(token)
	if err != nil || !link.IsActive || time.Now().After(link.ExpiresAt) {
		c.Status(http.StatusNotFound)
		return
	}

	passcode := c.PostForm("passcode")
	if passcode != link.Passcode {
		web.Render(c.Writer, "share_auth.html", map[string]any{
			"Token": token,
			"Error": "密码错误，请重试",
		})
		return
	}

	gw.renderShareView(c, link.PatientID)
}

func (gw *Server) renderShareView(c *gin.Context, patientID int) {
	profile, err := gw.store.GetPatientProfile(patientID)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	diagnoses, _ := gw.store.ListDiagnoses(patientID)
	meds, _ := gw.store.ListActiveMedications(patientID)
	abnormals, _ := gw.store.GetAbnormalIndicators(patientID)
	summaries, _ := gw.store.GetActiveMemorySummaries(patientID)

	web.Render(c.Writer, "share_view.html", map[string]any{
		"Profile":   profile,
		"Diagnoses": diagnoses,
		"Meds":      meds,
		"Abnormals": abnormals,
		"Summaries": summaries,
	})
}
