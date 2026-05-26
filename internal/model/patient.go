package model

import "time"

// PatientProfile represents the single local patient.
type PatientProfile struct {
	ID           int       `json:"id"`
	Name         string    `json:"name,omitempty"`
	Gender       string    `json:"gender,omitempty"` // male / female
	BirthDate    string    `json:"birth_date,omitempty"`
	Height       float64   `json:"height,omitempty"`  // cm
	Weight       float64   `json:"weight,omitempty"`  // kg
	Allergies    []string  `json:"allergies,omitempty"`
	MedicalNotes string    `json:"medical_notes,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Diagnosis records a diagnostic event linked to a health report.
type Diagnosis struct {
	ID             int       `json:"id"`
	DiagnosisDate  string    `json:"diagnosis_date"`
	Condition      string    `json:"condition"` // duodenal_ulcer / gastric_ulcer / hp_infection / ...
	Detail         string    `json:"detail"`    // JSON: ulcer location, size, stage, HP status, etc.
	SourceReportID *int      `json:"source_report_id,omitempty"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// MemorySummary stores AI-generated summaries injected into prompts.
type MemorySummary struct {
	ID             int       `json:"id"`
	Category       string    `json:"category"` // symptom_pattern / diet_pattern / medication_history / overall
	Content        string    `json:"content"`
	DataRangeStart string    `json:"data_range_start,omitempty"`
	DataRangeEnd   string    `json:"data_range_end,omitempty"`
	SupersededBy   *int      `json:"superseded_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// SymptomLog records a single symptom entry.
type SymptomLog struct {
	ID           int       `json:"id"`
	PainScore    *int      `json:"pain_score,omitempty"`    // 0-10
	PainLocation string    `json:"pain_location,omitempty"` // upper_abdomen / right_upper / navel_area
	PainTiming   string    `json:"pain_timing,omitempty"`   // fasting / postprandial / nocturnal
	StoolColor   string    `json:"stool_color,omitempty"`   // normal / dark / black / bloody
	Bloating     bool      `json:"bloating"`
	Nausea       bool      `json:"nausea"`
	AcidReflux   bool      `json:"acid_reflux"`
	Note         string    `json:"note,omitempty"`
	RecordedAt   time.Time `json:"recorded_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// MealLog records a single meal entry.
type MealLog struct {
	ID             int       `json:"id"`
	MealType       string    `json:"meal_type"` // breakfast / lunch / dinner / snack
	Content        string    `json:"content"`
	HasIrritant    bool      `json:"has_irritant"`
	IrritantDetail string    `json:"irritant_detail,omitempty"`
	RecordedAt     time.Time `json:"recorded_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// Medication represents a medication regimen.
type Medication struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category,omitempty"` // ppi / antibiotic / bismuth / other
	Dosage      string    `json:"dosage,omitempty"`
	Frequency   string    `json:"frequency,omitempty"`
	TimeOfDay   string    `json:"time_of_day,omitempty"`
	CourseStart string    `json:"course_start,omitempty"`
	CourseEnd   string    `json:"course_end,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// MedicationLog records whether a scheduled dose was taken.
type MedicationLog struct {
	ID           int       `json:"id"`
	MedicationID int       `json:"medication_id"`
	TakenAt      time.Time `json:"taken_at"`
	Skipped      bool      `json:"skipped"`
	Note         string    `json:"note,omitempty"`
}

// AIInsight caches an AI-generated insight.
type AIInsight struct {
	ID             int       `json:"id"`
	InsightType    string    `json:"insight_type"` // trend / report / diet / medication / emergency
	Content        string    `json:"content"`
	DataRangeStart string    `json:"data_range_start,omitempty"`
	DataRangeEnd   string    `json:"data_range_end,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
