package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
)

// SYNTHETIC DATA - not real patient information
func TestPhase4Integration(t *testing.T) {
	srv, pid := newTestServer(t)

	// Seed synthetic data for a richer test
	s := srv.store

	if _, err := s.InsertDiagnosis(&model.Diagnosis{
		PatientID:     pid,
		DiagnosisDate: "2026-01-10",
		Condition:     "十二指肠球部溃疡（SYNTHETIC）",
	}); err != nil {
		t.Fatalf("insert diagnosis: %v", err)
	}

	if _, err := s.InsertMedication(&model.Medication{
		PatientID: pid,
		Name:      "奥美拉唑（SYNTHETIC）",
		Dosage:    "20mg",
		Frequency: "BID",
		IsActive:  true,
	}); err != nil {
		t.Fatalf("insert medication: %v", err)
	}

	refLow := 4.0
	refHigh := 10.0
	if _, err := s.InsertIndicator(&model.MedicalIndicator{
		PatientID:         pid,
		Category:          "blood",
		IndicatorName:     "WBC",
		IndicatorNameCN:   "白细胞（SYNTHETIC）",
		Value:             "12.5",
		Unit:              "10^9/L",
		ReferenceLow:      &refLow,
		ReferenceHigh:     &refHigh,
		IsAbnormal:        true,
		AbnormalDirection:  "high",
		MeasuredAt:        "2026-05-20",
	}); err != nil {
		t.Fatalf("insert indicator: %v", err)
	}

	painScore := 5
	if _, err := s.InsertSymptomLog(&model.SymptomLog{
		PatientID:    pid,
		PainScore:    &painScore,
		PainLocation: "upper_abdomen",
		RecordedAt:   time.Now().Add(-24 * time.Hour),
	}); err != nil {
		t.Fatalf("insert symptom: %v", err)
	}

	if _, err := s.InsertAIInsight(&model.AIInsight{
		PatientID:   pid,
		InsightType: "trend_7d",
		Content:     "SYNTHETIC: 近7天症状平稳，建议继续观察。",
	}); err != nil {
		t.Fatalf("insert insight: %v", err)
	}

	// --- Step 1: Generate PDF ---
	t.Run("PDF_export", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d/summary/pdf", pid), nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("PDF status = %d, want 200; body: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "application/pdf" {
			t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
		}
		body := w.Body.Bytes()
		if string(body[:5]) != "%PDF-" {
			t.Error("not a valid PDF")
		}
		t.Logf("PDF generated: %d bytes", len(body))
	})

	// --- Step 2: Create share link with passcode ---
	var shareToken string
	t.Run("create_share_link", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader(`{"passcode":"4321"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", w.Code)
		}

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		shareToken = resp["token"].(string)
		if shareToken == "" {
			t.Fatal("empty token")
		}
		t.Logf("Share token: %s", shareToken[:8]+"...")
	})

	// --- Step 3: Access share view (should see password form) ---
	t.Run("share_view_auth_required", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/share/"+shareToken, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if !strings.Contains(w.Body.String(), "访问密码") {
			t.Error("expected password form")
		}
	})

	// --- Step 4: Verify with correct passcode ---
	t.Run("share_view_verify_correct", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/share/"+shareToken+"/verify", strings.NewReader("passcode=4321"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "家属只读视图") {
			t.Error("expected readonly share view")
		}
		if !strings.Contains(body, "Test Patient") {
			t.Error("expected patient name in share view")
		}
	})

	// --- Step 5: Verify wrong passcode rejected ---
	t.Run("share_view_verify_wrong", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/share/"+shareToken+"/verify", strings.NewReader("passcode=0000"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if !strings.Contains(w.Body.String(), "密码错误") {
			t.Error("expected error for wrong passcode")
		}
	})

	// --- Step 6: Invalid token returns 404 ---
	t.Run("share_view_invalid_token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/share/bogus-token-xyz", nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	// --- Step 7: Create no-passcode share link ---
	t.Run("share_no_passcode_flow", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		var resp map[string]any
		json.NewDecoder(w.Body).Decode(&resp)
		token := resp["token"].(string)

		req = httptest.NewRequest("GET", "/share/"+token, nil)
		w = httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		if !strings.Contains(w.Body.String(), "家属只读视图") {
			t.Error("expected direct access to readonly view without passcode")
		}
	})

	// --- Step 8: List share links ---
	t.Run("list_share_links", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d/shares", pid), nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var links []map[string]any
		json.NewDecoder(w.Body).Decode(&links)
		if len(links) < 2 {
			t.Errorf("expected at least 2 share links, got %d", len(links))
		}
	})

	// --- Step 9: Dashboard has Phase 4 nav elements ---
	t.Run("dashboard_phase4_elements", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d", pid), nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "生成复诊摘要") {
			t.Error("dashboard missing PDF export button")
		}
		if !strings.Contains(body, "分享给家属") {
			t.Error("dashboard missing share button")
		}
		if !strings.Contains(body, "v0.4") {
			t.Error("dashboard missing v0.4 version")
		}
		if !strings.Contains(body, `id="theme-toggle"`) {
			t.Error("dashboard missing theme toggle")
		}
		if !strings.Contains(body, `id="font-toggle"`) {
			t.Error("dashboard missing font toggle")
		}
	})
}
