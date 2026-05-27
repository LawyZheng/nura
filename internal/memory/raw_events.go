package memory

import (
	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/store"
)

// RawEvents provides access to raw fact records: reports, symptoms, medications.
type RawEvents struct {
	store *store.Store
}

func NewRawEvents(s *store.Store) *RawEvents {
	return &RawEvents{store: s}
}

func (r *RawEvents) ListReports(patientID int) ([]*model.HealthReport, error) {
	return r.store.ListHealthReports(patientID)
}

func (r *RawEvents) GetReport(id int) (*model.HealthReport, error) {
	return r.store.GetHealthReport(id)
}

func (r *RawEvents) AddReport(report *model.HealthReport) (int, error) {
	return r.store.InsertHealthReport(report)
}

func (r *RawEvents) ListSymptoms(patientID int, limit int) ([]*model.SymptomLog, error) {
	return r.store.ListSymptomLogs(patientID, limit)
}

func (r *RawEvents) AddSymptom(sl *model.SymptomLog) (int, error) {
	return r.store.InsertSymptomLog(sl)
}

func (r *RawEvents) ListActiveMedications(patientID int) ([]*model.Medication, error) {
	return r.store.ListActiveMedications(patientID)
}

func (r *RawEvents) ListDiagnoses(patientID int) ([]*model.Diagnosis, error) {
	return r.store.ListDiagnoses(patientID)
}

func (r *RawEvents) GetIndicatorsByReport(reportID int) ([]*model.MedicalIndicator, error) {
	return r.store.GetIndicatorsByReport(reportID)
}

func (r *RawEvents) GetIndicatorTimeline(patientID int, name string) ([]*model.MedicalIndicator, error) {
	return r.store.GetIndicatorTimeline(patientID, name)
}
