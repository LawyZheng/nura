package store

import (
	"fmt"

	"github.com/LawyZheng/nura/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store wraps a GORM database providing CRUD for all domain models.
type Store struct {
	db *gorm.DB
}

// New opens (or creates) an SQLite database at path and runs auto-migrations.
func New(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.Exec("PRAGMA journal_mode=WAL")

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// DB returns the underlying GORM database for advanced queries.
func (s *Store) DB() *gorm.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) migrate() error {
	return s.db.AutoMigrate(
		&model.PatientProfile{},
		&model.HealthReport{},
		&model.MedicalIndicator{},
		&model.Diagnosis{},
		&model.MemorySummary{},
		&model.SymptomLog{},
		&model.MealLog{},
		&model.Medication{},
		&model.MedicationLog{},
		&model.AIInsight{},
	)
}

// --- Patient Profile ---

func (s *Store) CreatePatientProfile(p *model.PatientProfile) (int, error) {
	if err := s.db.Create(p).Error; err != nil {
		return 0, err
	}
	return p.ID, nil
}

func (s *Store) UpdatePatientProfile(p *model.PatientProfile) error {
	return s.db.Save(p).Error
}

func (s *Store) GetPatientProfile(patientID int) (*model.PatientProfile, error) {
	var p model.PatientProfile
	if err := s.db.First(&p, patientID).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// --- Health Report ---

func (s *Store) InsertHealthReport(r *model.HealthReport) (int, error) {
	if err := s.db.Create(r).Error; err != nil {
		return 0, err
	}
	return r.ID, nil
}

func (s *Store) GetHealthReport(id int) (*model.HealthReport, error) {
	var r model.HealthReport
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) UpdateHealthReportProcessed(id int, aiType, aiSummary string) error {
	return s.db.Model(&model.HealthReport{}).Where("id = ?", id).Updates(map[string]any{
		"is_processed":       true,
		"ai_classified_type": aiType,
		"ai_summary":         aiSummary,
	}).Error
}

func (s *Store) ListHealthReports(patientID int) ([]*model.HealthReport, error) {
	var reports []*model.HealthReport
	if err := s.db.Where("patient_id = ?", patientID).Order("report_date DESC").Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}

// --- Medical Indicator ---

func (s *Store) InsertIndicator(ind *model.MedicalIndicator) (int, error) {
	result := s.db.Exec(`
		INSERT OR REPLACE INTO medical_indicators
		(patient_id, report_id, category, indicator_name, indicator_name_cn, value, unit,
		 reference_low, reference_high, is_abnormal, abnormal_direction, measured_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ind.PatientID, ind.ReportID, ind.Category, ind.IndicatorName, ind.IndicatorNameCN,
		ind.Value, ind.Unit, ind.ReferenceLow, ind.ReferenceHigh,
		ind.IsAbnormal, ind.AbnormalDirection, ind.MeasuredAt,
	)
	if result.Error != nil {
		return 0, result.Error
	}
	var inserted model.MedicalIndicator
	s.db.Where("patient_id = ? AND indicator_name = ? AND measured_at = ?", ind.PatientID, ind.IndicatorName, ind.MeasuredAt).First(&inserted)
	return inserted.ID, nil
}

func (s *Store) GetIndicatorsByReport(reportID int) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	if err := s.db.Where("report_id = ?", reportID).Order("category, indicator_name").Find(&indicators).Error; err != nil {
		return nil, err
	}
	return indicators, nil
}

func (s *Store) GetIndicatorTimeline(patientID int, indicatorName string) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	if err := s.db.Where("patient_id = ? AND indicator_name = ?", patientID, indicatorName).Order("measured_at").Find(&indicators).Error; err != nil {
		return nil, err
	}
	return indicators, nil
}

// --- Diagnosis ---

func (s *Store) InsertDiagnosis(d *model.Diagnosis) (int, error) {
	if err := s.db.Create(d).Error; err != nil {
		return 0, err
	}
	return d.ID, nil
}

func (s *Store) ListDiagnoses(patientID int) ([]*model.Diagnosis, error) {
	var diagnoses []*model.Diagnosis
	if err := s.db.Where("patient_id = ?", patientID).Order("diagnosis_date DESC").Find(&diagnoses).Error; err != nil {
		return nil, err
	}
	return diagnoses, nil
}

// --- Memory Summary ---

func (s *Store) InsertMemorySummary(m *model.MemorySummary) (int, error) {
	if err := s.db.Create(m).Error; err != nil {
		return 0, err
	}
	return m.ID, nil
}

func (s *Store) GetActiveMemorySummaries(patientID int) ([]*model.MemorySummary, error) {
	var summaries []*model.MemorySummary
	if err := s.db.Where("patient_id = ? AND superseded_by IS NULL", patientID).Order("category").Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

// --- Symptom Log ---

func (s *Store) InsertSymptomLog(sl *model.SymptomLog) (int, error) {
	if err := s.db.Create(sl).Error; err != nil {
		return 0, err
	}
	return sl.ID, nil
}

func (s *Store) ListSymptomLogs(patientID int, limit int) ([]*model.SymptomLog, error) {
	var logs []*model.SymptomLog
	if err := s.db.Where("patient_id = ?", patientID).Order("recorded_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// --- Meal Log ---

func (s *Store) InsertMealLog(ml *model.MealLog) (int, error) {
	if err := s.db.Create(ml).Error; err != nil {
		return 0, err
	}
	return ml.ID, nil
}

// --- Medication ---

func (s *Store) InsertMedication(m *model.Medication) (int, error) {
	if err := s.db.Create(m).Error; err != nil {
		return 0, err
	}
	return m.ID, nil
}

func (s *Store) ListActiveMedications(patientID int) ([]*model.Medication, error) {
	var meds []*model.Medication
	if err := s.db.Where("patient_id = ? AND is_active = ?", patientID, true).Order("course_start DESC").Find(&meds).Error; err != nil {
		return nil, err
	}
	return meds, nil
}

// --- Medication Log ---

func (s *Store) InsertMedicationLog(ml *model.MedicationLog) (int, error) {
	if err := s.db.Create(ml).Error; err != nil {
		return 0, err
	}
	return ml.ID, nil
}

func (s *Store) GetIndicatorsByCategory(patientID int, category string) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	if err := s.db.Where("patient_id = ? AND category = ?", patientID, category).Order("measured_at, indicator_name").Find(&indicators).Error; err != nil {
		return nil, err
	}
	return indicators, nil
}

func (s *Store) GetAbnormalIndicators(patientID int) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	if err := s.db.Where("patient_id = ? AND is_abnormal = ?", patientID, true).Order("measured_at DESC, indicator_name").Find(&indicators).Error; err != nil {
		return nil, err
	}
	return indicators, nil
}

func (s *Store) ListRecentReports(patientID int, limit int) ([]*model.HealthReport, error) {
	var reports []*model.HealthReport
	if err := s.db.Where("patient_id = ?", patientID).Order("report_date DESC").Limit(limit).Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}

// --- Patient Profile (list) ---

func (s *Store) ListPatientProfiles() ([]*model.PatientProfile, error) {
	var profiles []*model.PatientProfile
	if err := s.db.Order("id").Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

// --- Memory Summary (supersede) ---

func (s *Store) SupersedeMemorySummary(oldID, newID int) error {
	return s.db.Model(&model.MemorySummary{}).Where("id = ?", oldID).Update("superseded_by", newID).Error
}

// --- AI Insight ---

func (s *Store) InsertAIInsight(ai *model.AIInsight) (int, error) {
	if err := s.db.Create(ai).Error; err != nil {
		return 0, err
	}
	return ai.ID, nil
}
