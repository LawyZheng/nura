package gateway

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LawyZheng/nura/internal/pdf"
)

func (gw *Server) handleSummaryPDF(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid patient id"})
		return
	}

	profile, err := gw.store.GetPatientProfile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "patient not found"})
		return
	}

	svc := pdf.NewService(gw.store)
	pdfBytes, err := svc.GenerateSummary(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate pdf: %v", err)})
		return
	}

	filename := fmt.Sprintf("复诊摘要_%s.pdf", profile.Name)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
