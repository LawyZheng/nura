package store

import (
	"encoding/json"
	"fmt"

	"github.com/LawyZheng/nura/internal/model"
)

// PatientExport contains all data for a single patient, suitable for JSON backup.
type PatientExport struct {
	Profile    *model.PatientProfile    `json:"profile"`
	Reports    []*model.HealthReport    `json:"reports"`
	Indicators []*model.MedicalIndicator `json:"indicators"`
	Diagnoses  []*model.Diagnosis       `json:"diagnoses"`
	Symptoms   []*model.SymptomLog      `json:"symptoms"`
	Meals      []*model.MealLog         `json:"meals"`
	Medications []*model.Medication     `json:"medications"`
	MedLogs    []*model.MedicationLog   `json:"medication_logs"`
	Summaries  []*model.MemorySummary   `json:"memory_summaries"`
	Insights   []*model.AIInsight       `json:"insights"`
}

// ExportPatient dumps all data for the given patient as a PatientExport.
func (s *Store) ExportPatient(patientID int) (*PatientExport, error) {
	profile, err := s.GetPatientProfile(patientID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	var reports []*model.HealthReport
	s.db.Where("patient_id = ?", patientID).Order("report_date DESC").Find(&reports)

	var indicators []*model.MedicalIndicator
	s.db.Where("patient_id = ?", patientID).Order("measured_at, indicator_name").Find(&indicators)

	var diagnoses []*model.Diagnosis
	s.db.Where("patient_id = ?", patientID).Order("diagnosis_date DESC").Find(&diagnoses)

	var symptoms []*model.SymptomLog
	s.db.Where("patient_id = ?", patientID).Order("recorded_at DESC").Find(&symptoms)

	var meals []*model.MealLog
	s.db.Where("patient_id = ?", patientID).Order("recorded_at DESC").Find(&meals)

	var medications []*model.Medication
	s.db.Where("patient_id = ?", patientID).Order("course_start DESC").Find(&medications)

	// Medication logs: join through medication.patient_id.
	var medLogs []*model.MedicationLog
	s.db.Joins("JOIN medications ON medication_logs.medication_id = medications.id").
		Where("medications.patient_id = ?", patientID).
		Find(&medLogs)

	var summaries []*model.MemorySummary
	s.db.Where("patient_id = ?", patientID).Order("category").Find(&summaries)

	var insights []*model.AIInsight
	s.db.Where("patient_id = ?", patientID).Order("created_at DESC").Find(&insights)

	return &PatientExport{
		Profile:     profile,
		Reports:     reports,
		Indicators:  indicators,
		Diagnoses:   diagnoses,
		Symptoms:    symptoms,
		Meals:       meals,
		Medications: medications,
		MedLogs:     medLogs,
		Summaries:   summaries,
		Insights:    insights,
	}, nil
}

// ImportPatient restores patient data from an export. It creates a new patient
// profile (ignoring the exported ID) and re-links all records to the new ID.
func (s *Store) ImportPatient(patientID int, data *PatientExport) error {
	if data.Profile != nil {
		// Update the existing profile with exported data.
		profile := *data.Profile
		profile.ID = patientID
		if err := s.UpdatePatientProfile(&profile); err != nil {
			return fmt.Errorf("update profile: %w", err)
		}
	}

	// Build a mapping from old report IDs to new ones for FK re-linking.
	reportIDMap := make(map[int]int)
	for _, r := range data.Reports {
		oldID := r.ID
		r.ID = 0
		r.PatientID = patientID
		if err := s.db.Create(r).Error; err != nil {
			return fmt.Errorf("import report: %w", err)
		}
		reportIDMap[oldID] = r.ID
	}

	for _, ind := range data.Indicators {
		ind.ID = 0
		ind.PatientID = patientID
		if newRID, ok := reportIDMap[ind.ReportID]; ok {
			ind.ReportID = newRID
		}
		// Use raw insert to skip dedup issues during restore.
		s.db.Create(ind)
	}

	for _, d := range data.Diagnoses {
		d.ID = 0
		d.PatientID = patientID
		if d.SourceReportID != nil {
			if newRID, ok := reportIDMap[*d.SourceReportID]; ok {
				d.SourceReportID = &newRID
			}
		}
		s.db.Create(d)
	}

	for _, sl := range data.Symptoms {
		sl.ID = 0
		sl.PatientID = patientID
		s.db.Create(sl)
	}

	for _, ml := range data.Meals {
		ml.ID = 0
		ml.PatientID = patientID
		s.db.Create(ml)
	}

	// Medication import needs to map old medication IDs for med logs.
	medIDMap := make(map[int]int)
	for _, m := range data.Medications {
		oldID := m.ID
		m.ID = 0
		m.PatientID = patientID
		s.db.Create(m)
		medIDMap[oldID] = m.ID
	}

	for _, ml := range data.MedLogs {
		ml.ID = 0
		if newMID, ok := medIDMap[ml.MedicationID]; ok {
			ml.MedicationID = newMID
		}
		s.db.Create(ml)
	}

	for _, ms := range data.Summaries {
		ms.ID = 0
		ms.PatientID = patientID
		ms.SupersededBy = nil
		s.db.Create(ms)
	}

	for _, ai := range data.Insights {
		ai.ID = 0
		ai.PatientID = patientID
		s.db.Create(ai)
	}

	return nil
}

// ExportPatientJSON serializes patient data as indented JSON bytes.
func (s *Store) ExportPatientJSON(patientID int) ([]byte, error) {
	export, err := s.ExportPatient(patientID)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(export, "", "  ")
}
