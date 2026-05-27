package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateShareLink(t *testing.T) {
	srv, pid := newTestServer(t)

	body := `{"passcode":"1234"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected non-empty token")
	}
	if resp["url"] == nil || resp["url"] == "" {
		t.Error("expected non-empty url")
	}
}

func TestCreateShareLink_NoPasscode(t *testing.T) {
	srv, pid := newTestServer(t)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
}

func TestListShareLinks(t *testing.T) {
	srv, pid := newTestServer(t)

	// Create two links
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %d: status = %d", i, w.Code)
		}
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d/shares", pid), nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var links []map[string]any
	json.NewDecoder(w.Body).Decode(&links)
	if len(links) != 2 {
		t.Errorf("got %d links, want 2", len(links))
	}
}

func TestDeleteShareLink(t *testing.T) {
	srv, pid := newTestServer(t)

	// Create a link
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	shareID := int(resp["id"].(float64))

	// Delete it
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/patient/%d/share/%d", pid, shareID), nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}
