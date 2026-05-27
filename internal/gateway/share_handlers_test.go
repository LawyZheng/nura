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

func TestShareView_NoPasscode(t *testing.T) {
	srv, pid := newTestServer(t)

	// Create a share link without passcode
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	token := resp["token"].(string)

	// Access share view — should render directly
	req = httptest.NewRequest("GET", "/share/"+token, nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "家属只读视图") {
		t.Error("expected share view content with '家属只读视图'")
	}
}

func TestShareView_WithPasscode(t *testing.T) {
	srv, pid := newTestServer(t)

	// Create a share link with passcode
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader(`{"passcode":"5678"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	token := resp["token"].(string)

	// Access share view — should show auth form
	req = httptest.NewRequest("GET", "/share/"+token, nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "访问密码") {
		t.Error("expected password form")
	}

	// Verify with correct passcode
	req = httptest.NewRequest("POST", "/share/"+token+"/verify", strings.NewReader("passcode=5678"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "家属只读视图") {
		t.Error("expected share view after correct passcode")
	}
}

func TestShareView_WrongPasscode(t *testing.T) {
	srv, pid := newTestServer(t)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader(`{"passcode":"1111"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	token := resp["token"].(string)

	// Verify with wrong passcode
	req = httptest.NewRequest("POST", "/share/"+token+"/verify", strings.NewReader("passcode=9999"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "密码错误") {
		t.Error("expected error message for wrong passcode")
	}
}

func TestShareView_InvalidToken(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("GET", "/share/nonexistent-token", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
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
