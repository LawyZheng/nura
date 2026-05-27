package store

import (
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
)

func TestStore_CreateAndGetShareLink(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	link := &model.ShareLink{
		PatientID: pid,
		Token:     "test-token-abc123",
		Passcode:  "",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		IsActive:  true,
	}
	if err := s.CreateShareLink(link); err != nil {
		t.Fatalf("create share link: %v", err)
	}
	if link.ID <= 0 {
		t.Errorf("expected positive id, got %d", link.ID)
	}

	got, err := s.GetShareLinkByToken("test-token-abc123")
	if err != nil {
		t.Fatalf("get share link: %v", err)
	}
	if got.PatientID != pid {
		t.Errorf("patient_id = %d, want %d", got.PatientID, pid)
	}
	if got.Token != "test-token-abc123" {
		t.Errorf("token = %q, want %q", got.Token, "test-token-abc123")
	}
}

func TestStore_GetShareLinkByToken_NotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetShareLinkByToken("nonexistent-token")
	if err == nil {
		t.Fatal("expected error for nonexistent token, got nil")
	}
}

func TestStore_ListShareLinks(t *testing.T) {
	s := newTestStore(t)
	pid1 := createTestPatient(t, s)
	pid2, err := s.CreatePatientProfile(&model.PatientProfile{Name: "Other Patient"})
	if err != nil {
		t.Fatalf("create patient 2: %v", err)
	}

	for i, pid := range []int{pid1, pid1, pid2} {
		if err := s.CreateShareLink(&model.ShareLink{
			PatientID: pid,
			Token:     "token-" + string(rune('a'+i)),
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			IsActive:  true,
		}); err != nil {
			t.Fatalf("create link %d: %v", i, err)
		}
	}

	links, err := s.ListShareLinks(pid1)
	if err != nil {
		t.Fatalf("list share links: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("got %d links for pid1, want 2", len(links))
	}
}

func TestStore_DeactivateShareLink(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	link := &model.ShareLink{
		PatientID: pid,
		Token:     "token-to-deactivate",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		IsActive:  true,
	}
	if err := s.CreateShareLink(link); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := s.DeactivateShareLink(link.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	got, err := s.GetShareLinkByToken("token-to-deactivate")
	if err != nil {
		t.Fatalf("get after deactivate: %v", err)
	}
	if got.IsActive {
		t.Error("expected is_active=false after deactivation")
	}
}

func TestStore_ShareLink_WithPasscode(t *testing.T) {
	s := newTestStore(t)
	pid := createTestPatient(t, s)

	link := &model.ShareLink{
		PatientID: pid,
		Token:     "token-with-pass",
		Passcode:  "1234",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		IsActive:  true,
	}
	if err := s.CreateShareLink(link); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.GetShareLinkByToken("token-with-pass")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Passcode != "1234" {
		t.Errorf("passcode = %q, want %q", got.Passcode, "1234")
	}
}
