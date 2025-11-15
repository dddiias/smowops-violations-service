package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ViolationStatusLog struct {
	ID          uuid.UUID        `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ViolationID uuid.UUID        `gorm:"type:uuid;not null" json:"violation_id"`
	OldStatus   *ViolationStatus `gorm:"type:violation_status" json:"old_status"`
	NewStatus   ViolationStatus  `gorm:"type:violation_status;not null" json:"new_status"`
	Note        string           `gorm:"type:text" json:"note"`
	ChangedBy   *uuid.UUID       `gorm:"type:uuid" json:"changed_by"`
	CreatedAt   time.Time        `gorm:"autoCreateTime" json:"created_at"`
}

func (ViolationStatusLog) TableName() string {
	return "violation_status_log"
}

func (l *ViolationStatusLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

type AppealStatusLog struct {
	ID        uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	AppealID  uuid.UUID     `gorm:"type:uuid;not null" json:"appeal_id"`
	OldStatus *AppealStatus `gorm:"type:appeal_status" json:"old_status"`
	NewStatus AppealStatus  `gorm:"type:appeal_status;not null" json:"new_status"`
	Note      string        `gorm:"type:text" json:"note"`
	ChangedBy *uuid.UUID    `gorm:"type:uuid" json:"changed_by"`
	CreatedAt time.Time     `gorm:"autoCreateTime" json:"created_at"`
}

func (AppealStatusLog) TableName() string {
	return "appeal_status_log"
}

func (l *AppealStatusLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
