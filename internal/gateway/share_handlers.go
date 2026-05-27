package gateway

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/model"
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
