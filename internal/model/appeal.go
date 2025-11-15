package model

import (
	"time"

	"github.com/google/uuid"
)

type AppealStatus string

const (
	AppealStatusSubmitted   AppealStatus = "SUBMITTED"
	AppealStatusUnderReview AppealStatus = "UNDER_REVIEW"
	AppealStatusNeedInfo    AppealStatus = "NEED_INFO"
	AppealStatusApproved    AppealStatus = "APPROVED"
	AppealStatusRejected    AppealStatus = "REJECTED"
	AppealStatusClosed      AppealStatus = "CLOSED"
)

type AppealReasonCode string

const (
	AppealReasonCameraError     AppealReasonCode = "CAMERA_ERROR"
	AppealReasonTransitPath     AppealReasonCode = "TRANSIT_PATH"
	AppealReasonWrongAssignment AppealReasonCode = "WRONG_ASSIGNMENT"
	AppealReasonOther           AppealReasonCode = "OTHER"
)

type AttachmentFileType string

const (
	AttachmentFileImage AttachmentFileType = "IMAGE"
	AttachmentFileVideo AttachmentFileType = "VIDEO"
	AttachmentFileDoc   AttachmentFileType = "DOC"
)

type Appeal struct {
	ID           uuid.UUID        `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ViolationID  uuid.UUID        `gorm:"type:uuid;not null" json:"violation_id"`
	TripID       uuid.UUID        `gorm:"type:uuid;not null" json:"trip_id"`
	TicketID     *uuid.UUID       `gorm:"type:uuid" json:"ticket_id"`
	DriverID     *uuid.UUID       `gorm:"type:uuid" json:"driver_id"`
	ContractorID *uuid.UUID       `gorm:"type:uuid" json:"contractor_id"`
	ReasonCode   AppealReasonCode `gorm:"type:appeal_reason_code;not null" json:"reason_code"`
	ReasonText   string           `gorm:"type:text;not null" json:"reason_text"`
	Status       AppealStatus     `gorm:"type:appeal_status;not null;default:'SUBMITTED'" json:"status"`
	ResolvedBy   *uuid.UUID       `gorm:"type:uuid" json:"resolved_by"`
	ResolvedAt   *time.Time       `json:"resolved_at"`
	CreatedAt    time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time        `gorm:"autoUpdateTime" json:"updated_at"`

	Violation   *Violation         `gorm:"foreignKey:ViolationID"`
	Trip        *Trip              `gorm:"foreignKey:TripID"`
	Ticket      *Ticket            `gorm:"foreignKey:TicketID"`
	Driver      *Driver            `gorm:"foreignKey:DriverID"`
	Attachments []AppealAttachment `gorm:"foreignKey:AppealID"`
	Comments    []AppealComment    `gorm:"foreignKey:AppealID"`
}

func (Appeal) TableName() string {
	return "violation_appeals"
}

type AppealAttachment struct {
	ID         uuid.UUID          `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	AppealID   uuid.UUID          `gorm:"type:uuid;not null" json:"appeal_id"`
	FileURL    string             `gorm:"type:text;not null" json:"file_url"`
	FileType   AttachmentFileType `gorm:"type:attachment_file_type;not null" json:"file_type"`
	UploadedBy uuid.UUID          `gorm:"type:uuid;not null" json:"uploaded_by"`
	CreatedAt  time.Time          `gorm:"autoCreateTime" json:"created_at"`
}

func (AppealAttachment) TableName() string {
	return "violation_appeal_attachments"
}

type AppealComment struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	AppealID   uuid.UUID `gorm:"type:uuid;not null" json:"appeal_id"`
	AuthorID   uuid.UUID `gorm:"type:uuid;not null" json:"author_id"`
	AuthorRole UserRole  `gorm:"type:varchar(32);not null" json:"author_role"`
	Message    string    `gorm:"type:text;not null" json:"message"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AppealComment) TableName() string {
	return "violation_appeal_comments"
}
