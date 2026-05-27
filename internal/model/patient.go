package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringList is a []string that serializes to/from JSON in the database.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	data, err := json.Marshal(s)
	return string(data), err
}

func (s *StringList) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for StringList: %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// PatientProfile represents the single local patient.
type PatientProfile struct {
	ID           int        `json:"id" gorm:"primaryKey;check:id = 1"`
	Name         string     `json:"name,omitempty"`
	Gender       string     `json:"gender,omitempty"`
	BirthDate    string     `json:"birth_date,omitempty"`
	Height       float64    `json:"height,omitempty"`
	Weight       float64    `json:"weight,omitempty"`
	Allergies    StringList `json:"allergies,omitempty" gorm:"type:text"`
	MedicalNotes string     `json:"medical_notes,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// Diagnosis records a diagnostic event linked to a health report.
type Diagnosis struct {
	ID             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	DiagnosisDate  string    `json:"diagnosis_date" gorm:"not null"`
	Condition      string    `json:"condition" gorm:"not null"`
	Detail         string    `json:"detail"`
	SourceReportID *int      `json:"source_report_id,omitempty" gorm:"index"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// MemorySummary stores AI-generated summaries injected into prompts.
type MemorySummary struct {
	ID             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Category       string    `json:"category" gorm:"not null"`
	Content        string    `json:"content" gorm:"not null"`
	DataRangeStart string    `json:"data_range_start,omitempty"`
	DataRangeEnd   string    `json:"data_range_end,omitempty"`
	SupersededBy   *int      `json:"superseded_by,omitempty" gorm:"index"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// SymptomLog records a single symptom entry.
type SymptomLog struct {
	ID           int       `json:"id" gorm:"primaryKey;autoIncrement"`
	PainScore    *int      `json:"pain_score,omitempty"`
	PainLocation string    `json:"pain_location,omitempty"`
	PainTiming   string    `json:"pain_timing,omitempty"`
	StoolColor   string    `json:"stool_color,omitempty"`
	Bloating     bool      `json:"bloating" gorm:"default:false"`
	Nausea       bool      `json:"nausea" gorm:"default:false"`
	AcidReflux   bool      `json:"acid_reflux" gorm:"default:false"`
	Note         string    `json:"note,omitempty"`
	RecordedAt   time.Time `json:"recorded_at" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// MealLog records a single meal entry.
type MealLog struct {
	ID             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	MealType       string    `json:"meal_type" gorm:"not null"`
	Content        string    `json:"content" gorm:"not null"`
	HasIrritant    bool      `json:"has_irritant" gorm:"default:false"`
	IrritantDetail string    `json:"irritant_detail,omitempty"`
	RecordedAt     time.Time `json:"recorded_at" gorm:"not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// Medication represents a medication regimen.
type Medication struct {
	ID          int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"not null"`
	Category    string    `json:"category,omitempty"`
	Dosage      string    `json:"dosage,omitempty"`
	Frequency   string    `json:"frequency,omitempty"`
	TimeOfDay   string    `json:"time_of_day,omitempty"`
	CourseStart string    `json:"course_start,omitempty"`
	CourseEnd   string    `json:"course_end,omitempty"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// MedicationLog records whether a scheduled dose was taken.
type MedicationLog struct {
	ID           int       `json:"id" gorm:"primaryKey;autoIncrement"`
	MedicationID int       `json:"medication_id" gorm:"not null;index"`
	TakenAt      time.Time `json:"taken_at" gorm:"not null"`
	Skipped      bool      `json:"skipped" gorm:"default:false"`
	Note         string    `json:"note,omitempty"`
}

// AIInsight caches an AI-generated insight.
type AIInsight struct {
	ID             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	InsightType    string    `json:"insight_type" gorm:"not null"`
	Content        string    `json:"content" gorm:"not null"`
	DataRangeStart string    `json:"data_range_start,omitempty"`
	DataRangeEnd   string    `json:"data_range_end,omitempty"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}
