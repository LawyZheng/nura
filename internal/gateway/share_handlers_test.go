package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCreateShareLink(t *testing.T) {
	srv, pid := newTestServer(t)

	body := `{"passcode":"1234"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader(body))
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
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
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
		addOwnerSession(req, srv)
		addCSRFHeader(req, srv)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %d: status = %d", i, w.Code)
		}
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/patient/%d/shares", pid), nil)
	addOwnerSession(req, srv)
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
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
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
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
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
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
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
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	shareID := int(resp["id"].(float64))

	// Delete it
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/patient/%d/share/%d", pid, shareID), nil)
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func createShareWithPasscode(t *testing.T, srv *Server, pid int, passcode string) string {
	t.Helper()
	body := fmt.Sprintf(`{"passcode":"%s"}`, passcode)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/patient/%d/share", pid), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addOwnerSession(req, srv)
	addCSRFHeader(req, srv)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create share: status = %d, want 201", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	return resp["token"].(string)
}

func postVerify(srv *Server, token, passcode string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/share/"+token+"/verify", strings.NewReader("passcode="+passcode))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

func TestShareVerify_BruteForceProtection(t *testing.T) {
	srv, pid := newTestServer(t)
	token := createShareWithPasscode(t, srv, pid, "1234")

	for i := 1; i <= 5; i++ {
		w := postVerify(srv, token, "0000")
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: status = %d, want 200", i, w.Code)
		}
		if !strings.Contains(w.Body.String(), "密码错误") {
			t.Fatalf("attempt %d: expected '密码错误' in body", i)
		}
	}

	w := postVerify(srv, token, "0000")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 6: status = %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), "尝试次数过多") {
		t.Error("expected '尝试次数过多' in body")
	}
}

func TestShareVerify_CorrectPasscodeResetsCount(t *testing.T) {
	srv, pid := newTestServer(t)
	token := createShareWithPasscode(t, srv, pid, "1234")

	w := postVerify(srv, token, "0000")
	if w.Code != http.StatusOK {
		t.Fatalf("wrong attempt: status = %d, want 200", w.Code)
	}

	w = postVerify(srv, token, "1234")
	if w.Code != http.StatusOK {
		t.Fatalf("correct attempt: status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "家属只读视图") {
		t.Error("expected share view after correct passcode")
	}

	for i := 1; i <= 5; i++ {
		w = postVerify(srv, token, "0000")
		if w.Code != http.StatusOK {
			t.Fatalf("post-reset attempt %d: status = %d, want 200", i, w.Code)
		}
	}

	w = postVerify(srv, token, "0000")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("post-reset attempt 6: status = %d, want 429", w.Code)
	}
}

func TestShareVerify_LockedTokenRejectsCorrectPasscode(t *testing.T) {
	srv, pid := newTestServer(t)
	token := createShareWithPasscode(t, srv, pid, "1234")

	for i := 0; i < 5; i++ {
		postVerify(srv, token, "0000")
	}

	w := postVerify(srv, token, "1234")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("locked correct attempt: status = %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), "尝试次数过多") {
		t.Error("expected '尝试次数过多' even with correct passcode")
	}
}

func TestSharePasscodeLimiter_WindowExpiry(t *testing.T) {
	now := time.Now()
	limiter := newSharePasscodeLimiter(func() time.Time { return now })

	for i := 0; i < 5; i++ {
		limiter.recordFailure("tok1")
	}

	if !limiter.isLocked("tok1") {
		t.Fatal("expected token to be locked after 5 failures")
	}

	now = now.Add(16 * time.Minute)

	if limiter.isLocked("tok1") {
		t.Fatal("expected token to be unlocked after window expiry")
	}

	for i := 0; i < 5; i++ {
		limiter.recordFailure("tok1")
	}
	if !limiter.isLocked("tok1") {
		t.Fatal("expected token to be locked again after 5 new failures")
	}
}

func TestSharePasscodeLimiter_BurstAtomicity(t *testing.T) {
	limiter := newSharePasscodeLimiter(nil)

	normalCount := 0
	limitedCount := 0
	for i := 0; i < 10; i++ {
		if limiter.recordFailure("tok1") {
			limitedCount++
		} else {
			normalCount++
		}
	}

	if normalCount != 5 {
		t.Errorf("expected 5 normal attempts, got %d", normalCount)
	}
	if limitedCount != 5 {
		t.Errorf("expected 5 limited attempts, got %d", limitedCount)
	}
}

func TestShareVerify_ConcurrentBurstProtection(t *testing.T) {
	srv, pid := newTestServer(t)
	token := createShareWithPasscode(t, srv, pid, "1234")

	const n = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			w := postVerify(srv, token, "0000")
			codes[idx] = w.Code
		}(i)
	}

	close(start)
	wg.Wait()

	ok200 := 0
	too429 := 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			ok200++
		case http.StatusTooManyRequests:
			too429++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if ok200 > 5 {
		t.Errorf("got %d HTTP 200 responses, want at most 5", ok200)
	}
	if too429 < 1 {
		t.Error("expected at least one HTTP 429 response")
	}
}
