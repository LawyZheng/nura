package model

import "time"

// MedicalIndicator is a normalized indicator extracted from a health report.
type MedicalIndicator struct {
	ID                int       `json:"id"`
	ReportID          int       `json:"report_id"`
	Category          string    `json:"category"`           // gastroscopy / hp / blood / liver / kidney / stool
	IndicatorName     string    `json:"indicator_name"`     // hemoglobin / hp_status / ulcer_stage
	IndicatorNameCN   string    `json:"indicator_name_cn"`  // Chinese display name
	Value             string    `json:"value"`              // unified text (supports numeric and enum)
	Unit              string    `json:"unit,omitempty"`     // g/L, mmol/L, etc.
	ReferenceLow      *float64  `json:"reference_low,omitempty"`
	ReferenceHigh     *float64  `json:"reference_high,omitempty"`
	IsAbnormal        bool      `json:"is_abnormal"`
	AbnormalDirection string    `json:"abnormal_direction,omitempty"` // high / low / positive
	MeasuredAt        string    `json:"measured_at"`
	CreatedAt         time.Time `json:"created_at"`
}
