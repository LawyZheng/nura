package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
)

func addOwnerSession(req *http.Request, srv *Server) {
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: srv.ownerAuth.sessionToken,
	})
}

func addCSRFHeader(req *http.Request, srv *Server) {
	req.Header.Set(csrfHeaderName, srv.ownerAuth.csrfToken)
}

func TestOwnerAuth_UnauthenticatedGetReturns401(t *testing.T) {
	srv, pid := newTestServer(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for unauthenticated request", w.Code)
	}
}

func TestOwnerAuth_LoginInvalidToken(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/local/login?token=wrong-token", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for invalid bootstrap token", w.Code)
	}
}

func TestOwnerAuth_LoginValidTokenSetsCookiesAndRedirects(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/local/login?token="+srv.ownerAuth.bootstrapToken, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("status = %d, want 302 redirect", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("redirect location = %q, want /", loc)
	}

	cookies := w.Result().Cookies()
	var hasSession, hasCSRF bool
	for _, c := range cookies {
		switch c.Name {
		case sessionCookieName:
			hasSession = true
			if !c.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("session cookie SameSite = %d, want Lax (%d)", c.SameSite, http.SameSiteLaxMode)
			}
		case csrfCookieName:
			hasCSRF = true
			if c.HttpOnly {
				t.Error("CSRF cookie should NOT be HttpOnly (JS must read it)")
			}
		}
	}
	if !hasSession {
		t.Error("missing session cookie after valid login")
	}
	if !hasCSRF {
		t.Error("missing CSRF cookie after valid login")
	}
}

func TestOwnerAuth_AuthenticatedGetSucceeds(t *testing.T) {
	srv, pid := newTestServer(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d", pid), nil)
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for authenticated GET", w.Code)
	}
}

func TestOwnerAuth_PostWithoutCSRFReturns403(t *testing.T) {
	srv, pid := newTestServer(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id": pid,
		"pain_score": 3,
	})
	req := httptest.NewRequest("POST", "/api/symptoms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for POST without CSRF token", w.Code)
	}
}

func TestOwnerAuth_PostWithCSRFSucceeds(t *testing.T) {
	srv, pid := newTestServer(t)

	// SYNTHETIC DATA - not real patient information
	body, _ := json.Marshal(map[string]any{
		"patient_id":    pid,
		"pain_score":    3,
		"pain_location": "upper_abdomen",
	})
	req := httptest.NewRequest("POST", "/api/symptoms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201 for POST with valid session and CSRF", w.Code)
	}
}

func TestOwnerAuth_ShareViewWithoutSession(t *testing.T) {
	srv, pid := newTestServer(t)

	link := &model.ShareLink{
		PatientID: pid, Token: "test-share-no-auth",
		ExpiresAt: time.Now().Add(24 * time.Hour), IsActive: true,
	}
	srv.store.CreateShareLink(link)

	req := httptest.NewRequest("GET", "/share/test-share-no-auth", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Error("share view should not require owner session")
	}
}

func TestOwnerAuth_ShareVerifyWithoutSession(t *testing.T) {
	srv, pid := newTestServer(t)

	link := &model.ShareLink{
		PatientID: pid, Token: "test-verify-no-auth",
		Passcode:  "1234",
		ExpiresAt: time.Now().Add(24 * time.Hour), IsActive: true,
	}
	srv.store.CreateShareLink(link)

	req := httptest.NewRequest("POST", "/share/test-verify-no-auth/verify",
		strings.NewReader("passcode=1234"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Errorf("share verify should not require owner session or CSRF; got status %d", w.Code)
	}
}

func TestOwnerAuth_HealthWithoutSession(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for /health without session", w.Code)
	}
}

func TestOwnerAuth_CrossPatientStill404WhenAuthenticated(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/patient/999", nil)
	addOwnerSession(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for non-existent patient when authenticated", w.Code)
	}
}
