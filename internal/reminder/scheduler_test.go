package reminder

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestScheduler_GenerateDailyReminders_BID(t *testing.T) {
	s := newTestStore(t)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})

	// SYNTHETIC DATA - not real patient information
	s.InsertMedication(&model.Medication{
		PatientID:   pid,
		Name:        "Synthetic Omeprazole",
		Dosage:      "20mg",
		Frequency:   "bid",
		TimeOfDay:   "早晚餐前",
		CourseStart: "2026-05-20",
		CourseEnd:   "2026-06-02",
		IsActive:    true,
	})

	scheduler := NewScheduler(s)
	today := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	count, err := scheduler.GenerateDailyReminders(pid, today)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 reminders for bid, got %d", count)
	}

	before := time.Date(2026, 5, 25, 23, 59, 59, 0, time.UTC)
	pending, _ := s.ListPendingReminders(pid, before)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending reminders, got %d", len(pending))
	}
}

func TestScheduler_GenerateDailyReminders_TID(t *testing.T) {
	s := newTestStore(t)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})

	// SYNTHETIC DATA - not real patient information
	s.InsertMedication(&model.Medication{
		PatientID:   pid,
		Name:        "Synthetic Drug",
		Dosage:      "500mg",
		Frequency:   "tid",
		TimeOfDay:   "三餐后",
		CourseStart: "2026-05-20",
		CourseEnd:   "2026-06-02",
		IsActive:    true,
	})

	scheduler := NewScheduler(s)
	today := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	count, err := scheduler.GenerateDailyReminders(pid, today)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 reminders for tid, got %d", count)
	}
}

func TestScheduler_SkipsOutOfCourse(t *testing.T) {
	s := newTestStore(t)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})

	// SYNTHETIC DATA - not real patient information
	s.InsertMedication(&model.Medication{
		PatientID:   pid,
		Name:        "Synthetic Drug",
		Frequency:   "bid",
		CourseStart: "2026-05-20",
		CourseEnd:   "2026-05-25",
		IsActive:    true,
	})

	scheduler := NewScheduler(s)
	afterCourse := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	count, err := scheduler.GenerateDailyReminders(pid, afterCourse)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 reminders after course end, got %d", count)
	}
}

func TestScheduler_MarkDone(t *testing.T) {
	s := newTestStore(t)
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test Patient"})

	// SYNTHETIC DATA - not real patient information
	s.InsertMedication(&model.Medication{
		PatientID:   pid,
		Name:        "Synthetic Drug",
		Frequency:   "qd",
		CourseStart: "2026-05-20",
		CourseEnd:   "2026-06-02",
		IsActive:    true,
	})

	scheduler := NewScheduler(s)
	today := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	scheduler.GenerateDailyReminders(pid, today)

	before := time.Date(2026, 5, 25, 23, 59, 59, 0, time.UTC)
	pending, _ := s.ListPendingReminders(pid, before)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	s.MarkReminderDone(pending[0].ID)

	pending, _ = s.ListPendingReminders(pid, before)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after done, got %d", len(pending))
	}
}
