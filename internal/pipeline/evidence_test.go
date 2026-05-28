package pipeline

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveEvidence_WritesFileWithCorrectContentAndHash(t *testing.T) {
	dir := t.TempDir()
	// SYNTHETIC DATA - not real patient information
	content := "胃镜检查报告\n十二指肠球部：前壁可见一处溃疡"

	sourcePath, sourceHash, err := archiveEvidence(dir, 1, "2024-03-01", content)
	if err != nil {
		t.Fatalf("archiveEvidence: %v", err)
	}
	if sourcePath == "" || sourceHash == "" {
		t.Fatal("expected non-empty source_path and source_hash")
	}
	if sourceHash != fmt.Sprintf("%x", sha256.Sum256([]byte(content))) {
		t.Errorf("source_hash mismatch")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read archived file: %v", err)
	}
	if string(data) != content {
		t.Error("archived content does not match original")
	}
	if !strings.Contains(sourcePath, "patient_1") || !strings.Contains(sourcePath, "2024-03-01") {
		t.Errorf("source_path %q missing patient/date segments", sourcePath)
	}
	if !strings.HasSuffix(sourcePath, ".txt") {
		t.Error("expected .txt extension")
	}

	// Deterministic: same content → same hash.
	_, hash2, _ := archiveEvidence(dir, 1, "2024-03-01", content)
	if sourceHash != hash2 {
		t.Error("hashes differ for identical content")
	}
}

func TestArchiveEvidence_Permissions(t *testing.T) {
	dir := t.TempDir()
	sourcePath, _, err := archiveEvidence(dir, 1, "2024-06-15", "SYNTHETIC DATA: perm check")
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(sourcePath)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("file perm = %04o, want 0600", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Dir(sourcePath))
	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir perm = %04o, want 0700", di.Mode().Perm())
	}
}

func TestArchiveEvidence_FailsOnBadDir(t *testing.T) {
	// Use a regular file as archiveDir — deterministic failure.
	badPath := filepath.Join(t.TempDir(), "not-a-dir")
	os.WriteFile(badPath, []byte("block"), 0o644)

	_, _, err := archiveEvidence(badPath, 1, "2024-01-01", "SYNTHETIC DATA: test")
	if err == nil {
		t.Fatal("expected error for non-directory archive path")
	}
}

func TestArchiveEvidence_PathTraversalAndDateValidation(t *testing.T) {
	dir := t.TempDir()
	for _, date := range []string{"../../outside", "../etc/passwd", "not-a-date"} {
		if _, _, err := archiveEvidence(dir, 1, date, "SYNTHETIC DATA"); err == nil {
			t.Errorf("expected error for reportDate %q", date)
		}
	}
	// Empty date uses fallback.
	sp, _, err := archiveEvidence(dir, 1, "", "SYNTHETIC DATA: no date")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sp, "unknown-date") {
		t.Errorf("expected 'unknown-date' in path, got %q", sp)
	}
}
