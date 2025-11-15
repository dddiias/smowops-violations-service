package model

import (
	"time"

	"github.com/google/uuid"
)

type ViolationStatus string

const (
	ViolationStatusOpen     ViolationStatus = "OPEN"
	ViolationStatusCanceled ViolationStatus = "CANCELED"
	ViolationStatusFixed    ViolationStatus = "FIXED"
)

type ViolationSeverity string

const (
	ViolationSeverityLow    ViolationSeverity = "LOW"
	ViolationSeverityMedium ViolationSeverity = "MEDIUM"
	ViolationSeverityHigh   ViolationSeverity = "HIGH"
)

type ViolationDetectedBy string

const (
	ViolationDetectedByLpr    ViolationDetectedBy = "LPR"
	ViolationDetectedByVolume ViolationDetectedBy = "VOLUME"
	ViolationDetectedByGps    ViolationDetectedBy = "GPS"
	ViolationDetectedBySystem ViolationDetectedBy = "SYSTEM"
)

type ViolationType string

const (
	ViolationTypeRouteViolation  ViolationType = "ROUTE_VIOLATION"
	ViolationTypeForeignArea     ViolationType = "FOREIGN_AREA"
	ViolationTypeMismatchPlate   ViolationType = "MISMATCH_PLATE"
	ViolationTypeOverCapacity    ViolationType = "OVER_CAPACITY"
	ViolationTypeNoAreaWork      ViolationType = "NO_AREA_WORK"
	ViolationTypeOverContract    ViolationType = "OVER_CONTRACT_LIMIT"
	ViolationTypeSystemSuspicion ViolationType = "SYSTEM"
)

type Violation struct {
	ID          uuid.UUID           `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TripID      uuid.UUID           `gorm:"type:uuid;not null" json:"trip_id"`
	Type        ViolationType       `gorm:"type:varchar(64);not null" json:"type"`
	DetectedBy  ViolationDetectedBy `gorm:"type:violation_detected_by;not null" json:"detected_by"`
	Severity    ViolationSeverity   `gorm:"type:violation_severity;not null" json:"severity"`
	Status      ViolationStatus     `gorm:"type:violation_status;not null;default:'OPEN'" json:"status"`
	Description string              `gorm:"type:text" json:"description"`
	CreatedAt   time.Time           `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time           `gorm:"autoUpdateTime" json:"updated_at"`

	Trip *Trip `gorm:"foreignKey:TripID"`
}

func (Violation) TableName() string {
	return "violations"
}
