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
	ID               int        `json:"id"`
	ReportType       ReportType `json:"report_type"`
	ReportDate       string     `json:"report_date"`
	Institution      string     `json:"institution,omitempty"`
	RawText          string     `json:"raw_text"`
	AIClassifiedType string     `json:"ai_classified_type,omitempty"`
	AISummary        string     `json:"ai_summary,omitempty"`
	SourceType       string     `json:"source_type,omitempty"` // photo / pdf
	SourcePath       string     `json:"source_path,omitempty"`
	IsProcessed      bool       `json:"is_processed"`
	CreatedAt        time.Time  `json:"created_at"`
}
