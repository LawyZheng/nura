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

	// Enable WAL mode for better concurrent read performance.
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

func (s *Store) UpsertPatientProfile(p *model.PatientProfile) error {
	p.ID = 1
	return s.db.Save(p).Error
}

func (s *Store) GetPatientProfile() (*model.PatientProfile, error) {
	var p model.PatientProfile
	if err := s.db.First(&p, 1).Error; err != nil {
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
		"is_processed":      true,
		"ai_classified_type": aiType,
		"ai_summary":        aiSummary,
	}).Error
}

func (s *Store) ListHealthReports() ([]*model.HealthReport, error) {
	var reports []*model.HealthReport
	if err := s.db.Order("report_date DESC").Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}

// --- Medical Indicator ---

func (s *Store) InsertIndicator(ind *model.MedicalIndicator) (int, error) {
	// Use raw SQL for INSERT OR REPLACE to honor the unique dedup index.
	result := s.db.Exec(`
		INSERT OR REPLACE INTO medical_indicators
		(report_id, category, indicator_name, indicator_name_cn, value, unit,
		 reference_low, reference_high, is_abnormal, abnormal_direction, measured_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ind.ReportID, ind.Category, ind.IndicatorName, ind.IndicatorNameCN,
		ind.Value, ind.Unit, ind.ReferenceLow, ind.ReferenceHigh,
		ind.IsAbnormal, ind.AbnormalDirection, ind.MeasuredAt,
	)
	if result.Error != nil {
		return 0, result.Error
	}
	// Retrieve the ID of the inserted/replaced row.
	var inserted model.MedicalIndicator
	s.db.Where("indicator_name = ? AND measured_at = ?", ind.IndicatorName, ind.MeasuredAt).First(&inserted)
	return inserted.ID, nil
}

func (s *Store) GetIndicatorsByReport(reportID int) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	if err := s.db.Where("report_id = ?", reportID).Order("category, indicator_name").Find(&indicators).Error; err != nil {
		return nil, err
	}
	return indicators, nil
}

func (s *Store) GetIndicatorTimeline(indicatorName string) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	if err := s.db.Where("indicator_name = ?", indicatorName).Order("measured_at").Find(&indicators).Error; err != nil {
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

func (s *Store) ListDiagnoses() ([]*model.Diagnosis, error) {
	var diagnoses []*model.Diagnosis
	if err := s.db.Order("diagnosis_date DESC").Find(&diagnoses).Error; err != nil {
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

func (s *Store) GetActiveMemorySummaries() ([]*model.MemorySummary, error) {
	var summaries []*model.MemorySummary
	if err := s.db.Where("superseded_by IS NULL").Order("category").Find(&summaries).Error; err != nil {
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

func (s *Store) ListSymptomLogs(limit int) ([]*model.SymptomLog, error) {
	var logs []*model.SymptomLog
	if err := s.db.Order("recorded_at DESC").Limit(limit).Find(&logs).Error; err != nil {
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

func (s *Store) ListActiveMedications() ([]*model.Medication, error) {
	var meds []*model.Medication
	if err := s.db.Where("is_active = ?", true).Order("course_start DESC").Find(&meds).Error; err != nil {
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

// --- AI Insight ---

func (s *Store) InsertAIInsight(ai *model.AIInsight) (int, error) {
	if err := s.db.Create(ai).Error; err != nil {
		return 0, err
	}
	return ai.ID, nil
}
