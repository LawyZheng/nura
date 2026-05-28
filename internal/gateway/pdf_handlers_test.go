package gateway

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSummaryPDF_OK(t *testing.T) {
	srv, pid := newTestServer(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d/summary/pdf", pid), nil)
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if w.Header().Get("Content-Disposition") == "" {
		t.Error("expected Content-Disposition header")
	}
	body := w.Body.Bytes()
	if len(body) < 5 || string(body[:5]) != "%PDF-" {
		t.Errorf("expected PDF header, got %q", string(body[:min(10, len(body))]))
	}
}

func TestSummaryPDF_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/patient/99999/summary/pdf", nil)
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
