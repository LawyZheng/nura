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

func (r *RawEvents) ListReports() ([]*model.HealthReport, error) {
	return r.store.ListHealthReports()
}

func (r *RawEvents) GetReport(id int) (*model.HealthReport, error) {
	return r.store.GetHealthReport(id)
}

func (r *RawEvents) AddReport(report *model.HealthReport) (int, error) {
	return r.store.InsertHealthReport(report)
}

func (r *RawEvents) ListSymptoms(limit int) ([]*model.SymptomLog, error) {
	return r.store.ListSymptomLogs(limit)
}

func (r *RawEvents) AddSymptom(sl *model.SymptomLog) (int, error) {
	return r.store.InsertSymptomLog(sl)
}

func (r *RawEvents) ListActiveMedications() ([]*model.Medication, error) {
	return r.store.ListActiveMedications()
}

func (r *RawEvents) ListDiagnoses() ([]*model.Diagnosis, error) {
	return r.store.ListDiagnoses()
}

func (r *RawEvents) GetIndicatorsByReport(reportID int) ([]*model.MedicalIndicator, error) {
	return r.store.GetIndicatorsByReport(reportID)
}

func (r *RawEvents) GetIndicatorTimeline(name string) ([]*model.MedicalIndicator, error) {
	return r.store.GetIndicatorTimeline(name)
}
