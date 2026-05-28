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

// Security hardening regression tests.
// Added during security-hardening slice after Hermes PM enforced TDD discipline.

// --- A: CORS hardening ---

func TestCORS_AllowLocalhost(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("CORS origin = %q, want http://localhost:3000", got)
	}
}

func TestCORS_Allow127(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8000")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:8000" {
		t.Errorf("CORS origin = %q, want http://127.0.0.1:8000", got)
	}
}

func TestCORS_RejectArbitraryOrigin(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS origin = %q, want empty for arbitrary origin", got)
	}
}

func TestCORS_RejectSimilarHostname(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://localhost.evil.com")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS origin = %q, want empty for localhost.evil.com", got)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS origin = %q, want empty when no Origin sent", got)
	}
}

// --- B: Patient isolation — medication update ---

func TestAPI_UpdateMedication_WrongPatient(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	body := `{"is_active":false}`
	req := httptest.NewRequest("PUT",
		fmt.Sprintf("/api/medications/%d?patient_id=999", medID),
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for wrong patient", w.Code)
	}

	med, _ := s.GetMedication(medID)
	if !med.IsActive {
		t.Error("medication was mutated despite wrong patient_id")
	}
}

func TestAPI_UpdateMedication_MissingPatientID(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	body := `{"is_active":false}`
	req := httptest.NewRequest("PUT",
		fmt.Sprintf("/api/medications/%d", medID),
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when patient_id missing", w.Code)
	}
}

// --- B: Patient isolation — medication log create ---

func TestAPI_CreateMedicationLog_WrongPatient(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})

	body := `{"skipped":false,"note":"injected"}`
	req := httptest.NewRequest("POST",
		fmt.Sprintf("/api/medications/%d/log?patient_id=999", medID),
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for wrong patient on log create", w.Code)
	}

	logs, _ := s.ListMedicationLogs(medID)
	if len(logs) != 0 {
		t.Error("medication log was created despite wrong patient_id")
	}
}

// --- B: Patient isolation — medication log list ---

func TestAPI_ListMedicationLogs_WrongPatient(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})
	s.InsertMedicationLog(&model.MedicationLog{MedicationID: medID, TakenAt: time.Now()})

	req := httptest.NewRequest("GET",
		fmt.Sprintf("/api/medications/%d/logs?patient_id=999", medID), nil)
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for wrong patient on log list", w.Code)
	}
}

// --- B: Patient isolation — reminder done ---

func TestAPI_MarkReminderDone_WrongPatient(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})
	remID, _ := s.InsertMedicationReminder(&model.MedicationReminder{
		MedicationID: medID, PatientID: pid,
		ScheduledAt: time.Now().Add(-1 * time.Hour),
		Label:       "test reminder",
	})

	req := httptest.NewRequest("POST",
		fmt.Sprintf("/api/reminders/%d/done?patient_id=999", remID), nil)
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for wrong patient on reminder done", w.Code)
	}

	rem, _ := s.GetReminder(remID)
	if rem.IsDone {
		t.Error("reminder was marked done despite wrong patient_id")
	}
}

func TestAPI_MarkReminderDone_MissingPatientID(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	medID, _ := s.InsertMedication(&model.Medication{
		PatientID: pid, Name: "Synthetic Drug", IsActive: true,
	})
	remID, _ := s.InsertMedicationReminder(&model.MedicationReminder{
		MedicationID: medID, PatientID: pid,
		ScheduledAt: time.Now().Add(-1 * time.Hour),
		Label:       "test reminder",
	})

	req := httptest.NewRequest("POST",
		fmt.Sprintf("/api/reminders/%d/done", remID), nil)
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when patient_id missing", w.Code)
	}
}

// --- B: Patient isolation — share delete ---

func TestDeleteShareLink_WrongPatient(t *testing.T) {
	srv, s, pid := newTestServerWithData(t)

	// SYNTHETIC DATA - not real patient information
	link := &model.ShareLink{
		PatientID: pid, Token: "test-token-isolation",
		ExpiresAt: time.Now().Add(24 * time.Hour), IsActive: true,
	}
	s.CreateShareLink(link)

	req := httptest.NewRequest("DELETE",
		fmt.Sprintf("/api/patient/999/share/%d", link.ID), nil)
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for wrong patient on share delete", w.Code)
	}

	got, _ := s.GetShareLink(link.ID)
	if !got.IsActive {
		t.Error("share link was deactivated despite wrong patient_id")
	}
}

// --- C: Passcode hashing ---

func TestPasscodeNotStoredPlaintext(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"passcode":"1234"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", 1),
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	shareID := int(resp["id"].(float64))

	link, err := srv.store.GetShareLink(shareID)
	if err != nil {
		t.Fatalf("get share link: %v", err)
	}

	if link.Passcode == "1234" {
		t.Error("passcode stored as plaintext — must be hashed")
	}
	if link.Passcode == "" {
		t.Error("passcode field empty — hash should be stored")
	}
}

func TestPasscodeLegacyPlaintextVerification(t *testing.T) {
	srv, pid := newTestServer(t)

	// Simulate a pre-migration share link with plaintext passcode in DB
	link := &model.ShareLink{
		PatientID: pid, Token: "legacy-plaintext-token",
		Passcode:  "4321",
		ExpiresAt: time.Now().Add(24 * time.Hour), IsActive: true,
	}
	srv.store.CreateShareLink(link)

	// Should still be verifiable
	req := httptest.NewRequest("POST", "/share/legacy-plaintext-token/verify",
		strings.NewReader("passcode=4321"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("legacy verify: status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "家属只读视图") {
		t.Error("expected share view after correct legacy passcode")
	}
}

func TestPasscodeLegacyOpportunisticRehash(t *testing.T) {
	srv, pid := newTestServer(t)

	// Simulate a pre-migration share link with plaintext passcode
	link := &model.ShareLink{
		PatientID: pid, Token: "rehash-token",
		Passcode:  "9999",
		ExpiresAt: time.Now().Add(24 * time.Hour), IsActive: true,
	}
	srv.store.CreateShareLink(link)

	// Verify with correct passcode — triggers opportunistic rehash
	req := httptest.NewRequest("POST", "/share/rehash-token/verify",
		strings.NewReader("passcode=9999"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("legacy verify: status = %d, want 200", w.Code)
	}

	// After successful verify, passcode should be rehashed (no longer plaintext)
	updated, _ := srv.store.GetShareLink(link.ID)
	if updated.Passcode == "9999" {
		t.Error("passcode still plaintext after opportunistic rehash")
	}
	if !strings.Contains(updated.Passcode, "$") {
		t.Error("rehashed passcode should contain salt$hash separator")
	}

	// Verify still works with the rehashed value
	req = httptest.NewRequest("POST", "/share/rehash-token/verify",
		strings.NewReader("passcode=9999"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "家属只读视图") {
		t.Error("verification should still work after rehash")
	}
}

func TestPasscodeLegacyWrongRejected(t *testing.T) {
	srv, pid := newTestServer(t)

	link := &model.ShareLink{
		PatientID: pid, Token: "legacy-wrong-token",
		Passcode:  "1111",
		ExpiresAt: time.Now().Add(24 * time.Hour), IsActive: true,
	}
	srv.store.CreateShareLink(link)

	req := httptest.NewRequest("POST", "/share/legacy-wrong-token/verify",
		strings.NewReader("passcode=2222"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "密码错误") {
		t.Error("expected error for wrong passcode on legacy link")
	}
}

// --- Medication UI compatibility ---

func TestWeb_MedicationsPage_LogDoseIncludesPatientID(t *testing.T) {
	srv, _, pid := newTestServerWithData(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/patient/%d/medications", pid), nil)
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	expected := fmt.Sprintf("patient_id=%d", pid)
	if !strings.Contains(body, expected) {
		t.Errorf("medications page JS missing %q in logDose fetch URL", expected)
	}
}

// --- CORS Vary header ---

func TestCORS_VaryOriginHeader(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary header = %q, want to contain 'Origin'", got)
	}
}

func TestCORS_MalformedOrigin(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "://not-a-url")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS origin = %q, want empty for malformed origin", got)
	}
}

func TestPasscodeHashVerification(t *testing.T) {
	srv, pid := newTestServer(t)

	// Create share link with passcode
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid),
		strings.NewReader(`{"passcode":"5678"}`))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	token := resp["token"].(string)

	// Correct passcode should grant access
	req = httptest.NewRequest("POST", "/share/"+token+"/verify",
		strings.NewReader("passcode=5678"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("verify correct passcode: status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "家属只读视图") {
		t.Error("expected share view after correct passcode")
	}

	// Wrong passcode should be rejected
	req = httptest.NewRequest("POST", "/share/"+token+"/verify",
		strings.NewReader("passcode=0000"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "密码错误") {
		t.Error("expected error for wrong passcode")
	}
}
