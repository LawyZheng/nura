package reminder

import (
	"fmt"
	"strings"
	"time"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/store"
)

type Scheduler struct {
	store *store.Store
}

func NewScheduler(s *store.Store) *Scheduler {
	return &Scheduler{store: s}
}

// GenerateDailyReminders creates reminder entries for today for all active
// medications belonging to patientID. Existing undone reminders for today
// are skipped (idempotent).
func (s *Scheduler) GenerateDailyReminders(patientID int, date time.Time) (int, error) {
	meds, err := s.store.ListActiveMedications(patientID)
	if err != nil {
		return 0, err
	}

	today := date.Truncate(24 * time.Hour)
	count := 0

	for _, med := range meds {
		if !isWithinCourse(med, today) {
			continue
		}

		times := parseSchedule(med.Frequency, med.TimeOfDay, today)
		for _, t := range times {
			label := formatLabel(med, t)
			if _, err := s.store.InsertMedicationReminder(&model.MedicationReminder{
				MedicationID: med.ID,
				PatientID:    patientID,
				ScheduledAt:  t,
				Label:        label,
			}); err != nil {
				return count, err
			}
			count++
		}
	}

	return count, nil
}

func isWithinCourse(med *model.Medication, today time.Time) bool {
	if med.CourseStart == "" {
		return true
	}
	start, err := time.Parse("2006-01-02", med.CourseStart)
	if err != nil {
		return true
	}
	if today.Before(start) {
		return false
	}
	if med.CourseEnd != "" {
		end, err := time.Parse("2006-01-02", med.CourseEnd)
		if err == nil && today.After(end) {
			return false
		}
	}
	return true
}

func parseSchedule(frequency, timeOfDay string, today time.Time) []time.Time {
	year, month, day := today.Date()
	loc := today.Location()

	switch strings.ToLower(frequency) {
	case "bid":
		if strings.Contains(timeOfDay, "餐前") {
			return []time.Time{
				time.Date(year, month, day, 7, 30, 0, 0, loc),
				time.Date(year, month, day, 17, 30, 0, 0, loc),
			}
		}
		return []time.Time{
			time.Date(year, month, day, 8, 0, 0, 0, loc),
			time.Date(year, month, day, 18, 0, 0, 0, loc),
		}
	case "tid":
		return []time.Time{
			time.Date(year, month, day, 8, 0, 0, 0, loc),
			time.Date(year, month, day, 12, 30, 0, 0, loc),
			time.Date(year, month, day, 18, 0, 0, 0, loc),
		}
	case "qd":
		if strings.Contains(timeOfDay, "餐前") {
			return []time.Time{time.Date(year, month, day, 7, 30, 0, 0, loc)}
		}
		return []time.Time{time.Date(year, month, day, 8, 0, 0, 0, loc)}
	default:
		return []time.Time{time.Date(year, month, day, 8, 0, 0, 0, loc)}
	}
}

func formatLabel(med *model.Medication, t time.Time) string {
	timeStr := t.Format("15:04")
	return fmt.Sprintf("%s：%s %s", timeStr, med.Name, med.Dosage)
}
