package model

import "time"

// MedicalIndicator is a normalized indicator extracted from a health report.
type MedicalIndicator struct {
	ID                int       `json:"id" gorm:"primaryKey;autoIncrement"`
	ReportID          int       `json:"report_id" gorm:"not null;index"`
	Category          string    `json:"category" gorm:"not null"`
	IndicatorName     string    `json:"indicator_name" gorm:"not null;uniqueIndex:idx_indicator_dedup"`
	IndicatorNameCN   string    `json:"indicator_name_cn"`
	Value             string    `json:"value" gorm:"not null"`
	Unit              string    `json:"unit,omitempty"`
	ReferenceLow      *float64  `json:"reference_low,omitempty"`
	ReferenceHigh     *float64  `json:"reference_high,omitempty"`
	IsAbnormal        bool      `json:"is_abnormal" gorm:"default:false"`
	AbnormalDirection string    `json:"abnormal_direction,omitempty"`
	MeasuredAt        string    `json:"measured_at" gorm:"not null;uniqueIndex:idx_indicator_dedup"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
}
