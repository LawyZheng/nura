package pipeline

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func archiveEvidence(archiveDir string, patientID int, reportDate, content string) (sourcePath, sourceHash string, err error) {
	dateSegment, err := sanitizeDateSegment(reportDate)
	if err != nil {
		return "", "", err
	}

	hash := sha256.Sum256([]byte(content))
	sourceHash = fmt.Sprintf("%x", hash)

	dir := filepath.Join(archiveDir, fmt.Sprintf("patient_%d", patientID), dateSegment)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("create evidence directory: %w", err)
	}

	sourcePath = filepath.Join(dir, sourceHash+".txt")

	rel, err := filepath.Rel(archiveDir, sourcePath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("evidence path escapes archive directory")
	}

	if err := os.WriteFile(sourcePath, []byte(content), 0o600); err != nil {
		return "", "", fmt.Errorf("write evidence file: %w", err)
	}

	return sourcePath, sourceHash, nil
}

func sanitizeDateSegment(reportDate string) (string, error) {
	if reportDate == "" {
		return "unknown-date", nil
	}
	parsed, err := time.Parse("2006-01-02", reportDate)
	if err != nil {
		return "", fmt.Errorf("invalid report_date %q: %w", reportDate, err)
	}
	return parsed.Format("2006-01-02"), nil
}
