package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LawyZheng/nura/internal/model"

	_ "modernc.org/sqlite"
)

// Store wraps an SQLite database providing CRUD for all domain models.
type Store struct {
	db *sql.DB
}

// New opens (or creates) an SQLite database at path and runs migrations.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS patient_profile (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			name TEXT,
			gender TEXT,
			birth_date DATE,
			height REAL,
			weight REAL,
			allergies TEXT,
			medical_notes TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS health_report (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_type TEXT NOT NULL,
			report_date DATE NOT NULL,
			institution TEXT,
			raw_text TEXT,
			ai_classified_type TEXT,
			ai_summary TEXT,
			source_type TEXT,
			source_path TEXT,
			is_processed BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS medical_indicator (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			report_id INTEGER NOT NULL,
			category TEXT NOT NULL,
			indicator_name TEXT NOT NULL,
			indicator_name_cn TEXT,
			value TEXT NOT NULL,
			unit TEXT,
			reference_low REAL,
			reference_high REAL,
			is_abnormal BOOLEAN DEFAULT 0,
			abnormal_direction TEXT,
			measured_at DATE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (report_id) REFERENCES health_report(id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_indicator_dedup
			ON medical_indicator(indicator_name, measured_at)`,
		`CREATE TABLE IF NOT EXISTS diagnosis (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			diagnosis_date DATE NOT NULL,
			condition TEXT NOT NULL,
			detail TEXT,
			source_report_id INTEGER,
			note TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_report_id) REFERENCES health_report(id)
		)`,
		`CREATE TABLE IF NOT EXISTS memory_summary (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			content TEXT NOT NULL,
			data_range_start DATE,
			data_range_end DATE,
			superseded_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS symptom_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pain_score INTEGER,
			pain_location TEXT,
			pain_timing TEXT,
			stool_color TEXT,
			bloating BOOLEAN DEFAULT 0,
			nausea BOOLEAN DEFAULT 0,
			acid_reflux BOOLEAN DEFAULT 0,
			note TEXT,
			recorded_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS meal_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			meal_type TEXT NOT NULL,
			content TEXT NOT NULL,
			has_irritant BOOLEAN DEFAULT 0,
			irritant_detail TEXT,
			recorded_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS medication (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			category TEXT,
			dosage TEXT,
			frequency TEXT,
			time_of_day TEXT,
			course_start DATE,
			course_end DATE,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS medication_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			medication_id INTEGER NOT NULL,
			taken_at DATETIME NOT NULL,
			skipped BOOLEAN DEFAULT 0,
			note TEXT,
			FOREIGN KEY (medication_id) REFERENCES medication(id)
		)`,
		`CREATE TABLE IF NOT EXISTS ai_insight (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			insight_type TEXT NOT NULL,
			content TEXT NOT NULL,
			data_range_start DATE,
			data_range_end DATE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

// --- Patient Profile ---

func (s *Store) UpsertPatientProfile(p *model.PatientProfile) error {
	allergiesJSON, _ := json.Marshal(p.Allergies)
	_, err := s.db.Exec(`
		INSERT INTO patient_profile (id, name, gender, birth_date, height, weight, allergies, medical_notes, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, gender=excluded.gender, birth_date=excluded.birth_date,
			height=excluded.height, weight=excluded.weight, allergies=excluded.allergies,
			medical_notes=excluded.medical_notes, updated_at=excluded.updated_at`,
		p.Name, p.Gender, p.BirthDate, p.Height, p.Weight,
		string(allergiesJSON), p.MedicalNotes, time.Now(),
	)
	return err
}

func (s *Store) GetPatientProfile() (*model.PatientProfile, error) {
	row := s.db.QueryRow(`SELECT id, name, gender, birth_date, height, weight, allergies, medical_notes, updated_at FROM patient_profile WHERE id=1`)
	p := &model.PatientProfile{}
	var allergiesStr sql.NullString
	var name, gender, birthDate, notes sql.NullString
	err := row.Scan(&p.ID, &name, &gender, &birthDate, &p.Height, &p.Weight, &allergiesStr, &notes, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Name = name.String
	p.Gender = gender.String
	p.BirthDate = birthDate.String
	p.MedicalNotes = notes.String
	if allergiesStr.Valid {
		_ = json.Unmarshal([]byte(allergiesStr.String), &p.Allergies)
	}
	return p, nil
}

// --- Health Report ---

func (s *Store) InsertHealthReport(r *model.HealthReport) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO health_report (report_type, report_date, institution, raw_text, ai_classified_type, ai_summary, source_type, source_path, is_processed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(r.ReportType), r.ReportDate, r.Institution, r.RawText,
		r.AIClassifiedType, r.AISummary, r.SourceType, r.SourcePath, r.IsProcessed,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) GetHealthReport(id int) (*model.HealthReport, error) {
	row := s.db.QueryRow(`SELECT id, report_type, report_date, institution, raw_text, ai_classified_type, ai_summary, source_type, source_path, is_processed, created_at FROM health_report WHERE id=?`, id)
	r := &model.HealthReport{}
	var rt string
	var inst, aiType, aiSum, srcType, srcPath sql.NullString
	err := row.Scan(&r.ID, &rt, &r.ReportDate, &inst, &r.RawText, &aiType, &aiSum, &srcType, &srcPath, &r.IsProcessed, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	r.ReportType = model.ReportType(rt)
	r.Institution = inst.String
	r.AIClassifiedType = aiType.String
	r.AISummary = aiSum.String
	r.SourceType = srcType.String
	r.SourcePath = srcPath.String
	return r, nil
}

func (s *Store) UpdateHealthReportProcessed(id int, aiType, aiSummary string) error {
	_, err := s.db.Exec(`UPDATE health_report SET is_processed=1, ai_classified_type=?, ai_summary=? WHERE id=?`, aiType, aiSummary, id)
	return err
}

func (s *Store) ListHealthReports() ([]*model.HealthReport, error) {
	rows, err := s.db.Query(`SELECT id, report_type, report_date, institution, raw_text, ai_classified_type, ai_summary, source_type, source_path, is_processed, created_at FROM health_report ORDER BY report_date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*model.HealthReport
	for rows.Next() {
		r := &model.HealthReport{}
		var rt string
		var inst, aiType, aiSum, srcType, srcPath sql.NullString
		if err := rows.Scan(&r.ID, &rt, &r.ReportDate, &inst, &r.RawText, &aiType, &aiSum, &srcType, &srcPath, &r.IsProcessed, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.ReportType = model.ReportType(rt)
		r.Institution = inst.String
		r.AIClassifiedType = aiType.String
		r.AISummary = aiSum.String
		r.SourceType = srcType.String
		r.SourcePath = srcPath.String
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// --- Medical Indicator ---

func (s *Store) InsertIndicator(ind *model.MedicalIndicator) (int, error) {
	res, err := s.db.Exec(`
		INSERT OR REPLACE INTO medical_indicator (report_id, category, indicator_name, indicator_name_cn, value, unit, reference_low, reference_high, is_abnormal, abnormal_direction, measured_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ind.ReportID, ind.Category, ind.IndicatorName, ind.IndicatorNameCN,
		ind.Value, ind.Unit, ind.ReferenceLow, ind.ReferenceHigh,
		ind.IsAbnormal, ind.AbnormalDirection, ind.MeasuredAt,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) GetIndicatorsByReport(reportID int) ([]*model.MedicalIndicator, error) {
	rows, err := s.db.Query(`SELECT id, report_id, category, indicator_name, indicator_name_cn, value, unit, reference_low, reference_high, is_abnormal, abnormal_direction, measured_at, created_at FROM medical_indicator WHERE report_id=? ORDER BY category, indicator_name`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndicators(rows)
}

func (s *Store) GetIndicatorTimeline(indicatorName string) ([]*model.MedicalIndicator, error) {
	rows, err := s.db.Query(`SELECT id, report_id, category, indicator_name, indicator_name_cn, value, unit, reference_low, reference_high, is_abnormal, abnormal_direction, measured_at, created_at FROM medical_indicator WHERE indicator_name=? ORDER BY measured_at`, indicatorName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIndicators(rows)
}

func scanIndicators(rows *sql.Rows) ([]*model.MedicalIndicator, error) {
	var indicators []*model.MedicalIndicator
	for rows.Next() {
		ind := &model.MedicalIndicator{}
		var unit, abnDir, nameCN sql.NullString
		var refLow, refHigh sql.NullFloat64
		if err := rows.Scan(&ind.ID, &ind.ReportID, &ind.Category, &ind.IndicatorName,
			&nameCN, &ind.Value, &unit, &refLow, &refHigh,
			&ind.IsAbnormal, &abnDir, &ind.MeasuredAt, &ind.CreatedAt); err != nil {
			return nil, err
		}
		ind.IndicatorNameCN = nameCN.String
		ind.Unit = unit.String
		ind.AbnormalDirection = abnDir.String
		if refLow.Valid {
			ind.ReferenceLow = &refLow.Float64
		}
		if refHigh.Valid {
			ind.ReferenceHigh = &refHigh.Float64
		}
		indicators = append(indicators, ind)
	}
	return indicators, rows.Err()
}

// --- Diagnosis ---

func (s *Store) InsertDiagnosis(d *model.Diagnosis) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO diagnosis (diagnosis_date, condition, detail, source_report_id, note)
		VALUES (?, ?, ?, ?, ?)`,
		d.DiagnosisDate, d.Condition, d.Detail, d.SourceReportID, d.Note,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) ListDiagnoses() ([]*model.Diagnosis, error) {
	rows, err := s.db.Query(`SELECT id, diagnosis_date, condition, detail, source_report_id, note, created_at FROM diagnosis ORDER BY diagnosis_date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var diagnoses []*model.Diagnosis
	for rows.Next() {
		d := &model.Diagnosis{}
		var detail, note sql.NullString
		var srcID sql.NullInt64
		if err := rows.Scan(&d.ID, &d.DiagnosisDate, &d.Condition, &detail, &srcID, &note, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Detail = detail.String
		d.Note = note.String
		if srcID.Valid {
			v := int(srcID.Int64)
			d.SourceReportID = &v
		}
		diagnoses = append(diagnoses, d)
	}
	return diagnoses, rows.Err()
}

// --- Memory Summary ---

func (s *Store) InsertMemorySummary(m *model.MemorySummary) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO memory_summary (category, content, data_range_start, data_range_end)
		VALUES (?, ?, ?, ?)`,
		m.Category, m.Content, m.DataRangeStart, m.DataRangeEnd,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) GetActiveMemorySummaries() ([]*model.MemorySummary, error) {
	rows, err := s.db.Query(`SELECT id, category, content, data_range_start, data_range_end, superseded_by, created_at FROM memory_summary WHERE superseded_by IS NULL ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var summaries []*model.MemorySummary
	for rows.Next() {
		m := &model.MemorySummary{}
		var start, end sql.NullString
		var sup sql.NullInt64
		if err := rows.Scan(&m.ID, &m.Category, &m.Content, &start, &end, &sup, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.DataRangeStart = start.String
		m.DataRangeEnd = end.String
		if sup.Valid {
			v := int(sup.Int64)
			m.SupersededBy = &v
		}
		summaries = append(summaries, m)
	}
	return summaries, rows.Err()
}

// --- Symptom Log ---

func (s *Store) InsertSymptomLog(sl *model.SymptomLog) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO symptom_log (pain_score, pain_location, pain_timing, stool_color, bloating, nausea, acid_reflux, note, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sl.PainScore, sl.PainLocation, sl.PainTiming, sl.StoolColor,
		sl.Bloating, sl.Nausea, sl.AcidReflux, sl.Note, sl.RecordedAt,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) ListSymptomLogs(limit int) ([]*model.SymptomLog, error) {
	rows, err := s.db.Query(`SELECT id, pain_score, pain_location, pain_timing, stool_color, bloating, nausea, acid_reflux, note, recorded_at, created_at FROM symptom_log ORDER BY recorded_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*model.SymptomLog
	for rows.Next() {
		sl := &model.SymptomLog{}
		var ps sql.NullInt64
		var loc, timing, color, note sql.NullString
		if err := rows.Scan(&sl.ID, &ps, &loc, &timing, &color, &sl.Bloating, &sl.Nausea, &sl.AcidReflux, &note, &sl.RecordedAt, &sl.CreatedAt); err != nil {
			return nil, err
		}
		if ps.Valid {
			v := int(ps.Int64)
			sl.PainScore = &v
		}
		sl.PainLocation = loc.String
		sl.PainTiming = timing.String
		sl.StoolColor = color.String
		sl.Note = note.String
		logs = append(logs, sl)
	}
	return logs, rows.Err()
}

// --- Meal Log ---

func (s *Store) InsertMealLog(ml *model.MealLog) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO meal_log (meal_type, content, has_irritant, irritant_detail, recorded_at)
		VALUES (?, ?, ?, ?, ?)`,
		ml.MealType, ml.Content, ml.HasIrritant, ml.IrritantDetail, ml.RecordedAt,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

// --- Medication ---

func (s *Store) InsertMedication(m *model.Medication) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO medication (name, category, dosage, frequency, time_of_day, course_start, course_end, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Name, m.Category, m.Dosage, m.Frequency, m.TimeOfDay,
		m.CourseStart, m.CourseEnd, m.IsActive,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) ListActiveMedications() ([]*model.Medication, error) {
	rows, err := s.db.Query(`SELECT id, name, category, dosage, frequency, time_of_day, course_start, course_end, is_active, created_at FROM medication WHERE is_active=1 ORDER BY course_start DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var meds []*model.Medication
	for rows.Next() {
		m := &model.Medication{}
		var cat, dosage, freq, tod, start, end sql.NullString
		if err := rows.Scan(&m.ID, &m.Name, &cat, &dosage, &freq, &tod, &start, &end, &m.IsActive, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Category = cat.String
		m.Dosage = dosage.String
		m.Frequency = freq.String
		m.TimeOfDay = tod.String
		m.CourseStart = start.String
		m.CourseEnd = end.String
		meds = append(meds, m)
	}
	return meds, rows.Err()
}

// --- Medication Log ---

func (s *Store) InsertMedicationLog(ml *model.MedicationLog) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO medication_log (medication_id, taken_at, skipped, note)
		VALUES (?, ?, ?, ?)`,
		ml.MedicationID, ml.TakenAt, ml.Skipped, ml.Note,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

// --- AI Insight ---

func (s *Store) InsertAIInsight(ai *model.AIInsight) (int, error) {
	res, err := s.db.Exec(`
		INSERT INTO ai_insight (insight_type, content, data_range_start, data_range_end)
		VALUES (?, ?, ?, ?)`,
		ai.InsightType, ai.Content, ai.DataRangeStart, ai.DataRangeEnd,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}
