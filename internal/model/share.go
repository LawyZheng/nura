package model

import "time"

type ShareLink struct {
	ID        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	PatientID int       `json:"patient_id" gorm:"index;not null"`
	Token     string    `json:"token" gorm:"uniqueIndex;size:64;not null"`
	Passcode  string    `json:"-" gorm:"size:128"`
	ExpiresAt time.Time `json:"expires_at"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}
