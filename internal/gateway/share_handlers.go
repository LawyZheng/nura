package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
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

func hashPasscode(passcode string) (string, error) {
	if passcode == "" {
		return "", nil
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(passcode))
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(h.Sum(nil)), nil
}

func isHashedPasscode(stored string) bool {
	return strings.Contains(stored, "$")
}

func verifyPasscode(stored, input string) bool {
	if stored == "" && input == "" {
		return true
	}
	if !isHashedPasscode(stored) {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(input)) == 1
	}
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(input))
	return subtle.ConstantTimeCompare(expected, h.Sum(nil)) == 1
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

	hashed, err := hashPasscode(req.Passcode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash passcode"})
		return
	}

	link := &model.ShareLink{
		PatientID: pid,
		Token:     token,
		Passcode:  hashed,
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
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient id"})
		return
	}

	shareID, err := strconv.Atoi(c.Param("share_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share id"})
		return
	}

	link, err := gw.store.GetShareLink(shareID)
	if err != nil || link.PatientID != pid {
		c.JSON(http.StatusNotFound, gin.H{"error": "share link not found"})
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

	if gw.passcodeLimit.isLocked(token) {
		c.Status(http.StatusTooManyRequests)
		web.Render(c.Writer, "share_auth.html", map[string]any{
			"Token": token,
			"Error": "尝试次数过多，请稍后再试",
		})
		return
	}

	passcode := c.PostForm("passcode")
	if !verifyPasscode(link.Passcode, passcode) {
		if gw.passcodeLimit.recordFailure(token) {
			c.Status(http.StatusTooManyRequests)
			web.Render(c.Writer, "share_auth.html", map[string]any{
				"Token": token,
				"Error": "尝试次数过多，请稍后再试",
			})
			return
		}
		web.Render(c.Writer, "share_auth.html", map[string]any{
			"Token": token,
			"Error": "密码错误，请重试",
		})
		return
	}

	gw.passcodeLimit.resetOnSuccess(token)

	if !isHashedPasscode(link.Passcode) {
		if hashed, err := hashPasscode(passcode); err == nil {
			gw.store.UpdateShareLinkPasscode(link.ID, hashed)
		}
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
