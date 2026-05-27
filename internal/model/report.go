package model

import "time"

// ReportType enumerates supported health report types.
type ReportType string

const (
	ReportGastroscopy      ReportType = "gastroscopy"
	ReportHPBreath         ReportType = "hp_breath"
	ReportHPAntibody       ReportType = "hp_antibody"
	ReportBloodRoutine     ReportType = "blood_routine"
	ReportLiverFunction    ReportType = "liver_function"
	ReportKidneyFunction   ReportType = "kidney_function"
	ReportStoolRoutine     ReportType = "stool_routine"
	ReportStoolOccultBlood ReportType = "stool_occult_blood"
	ReportGeneralCheckup   ReportType = "general_checkup"
	ReportUnknown          ReportType = "unknown"
)

// HealthReport stores a raw health report and its processing state.
type HealthReport struct {
	ID               int        `json:"id" gorm:"primaryKey;autoIncrement"`
	PatientID        int        `json:"patient_id" gorm:"not null;index"`
	ReportType       ReportType `json:"report_type" gorm:"type:text;not null"`
	ReportDate       string     `json:"report_date" gorm:"not null"`
	Institution      string     `json:"institution,omitempty"`
	RawText          string     `json:"raw_text"`
	AIClassifiedType string     `json:"ai_classified_type,omitempty"`
	AISummary        string     `json:"ai_summary,omitempty"`
	SourceType       string     `json:"source_type,omitempty"`
	SourcePath       string     `json:"source_path,omitempty"`
	IsProcessed      bool       `json:"is_processed" gorm:"default:false"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
}
