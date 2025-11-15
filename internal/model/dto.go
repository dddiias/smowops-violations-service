package model

import (
	"time"

	"github.com/google/uuid"
)

type OrgBrief struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type DriverBrief struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"full_name"`
	Phone    string    `json:"phone"`
}

type VehicleBrief struct {
	ID          uuid.UUID `json:"id"`
	PlateNumber string    `json:"plate_number"`
	Brand       string    `json:"brand"`
	Model       string    `json:"model"`
}

type AreaBrief struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type TicketBrief struct {
	ID        uuid.UUID  `json:"id"`
	Status    string     `json:"status"`
	StartPlan *time.Time `json:"planned_start_at,omitempty"`
	EndPlan   *time.Time `json:"planned_end_at,omitempty"`
}

type ViolationRecord struct {
	Violation           Violation     `json:"violation"`
	TripStatus          string        `json:"trip_status"`
	TripEntryAt         *time.Time    `json:"trip_entry_at"`
	TripViolationReason *string       `json:"trip_violation_reason"`
	Contractor          *OrgBrief     `json:"contractor"`
	Ticket              *TicketBrief  `json:"ticket"`
	Driver              *DriverBrief  `json:"driver"`
	Vehicle             *VehicleBrief `json:"vehicle"`
	Area                *AreaBrief    `json:"cleaning_area"`
	PolygonName         *string       `json:"polygon_name"`
	LastAppeal          *AppealBrief  `json:"last_appeal"`
	HasActiveAppeal     bool          `json:"has_active_appeal"`
}

type AppealBrief struct {
	ID         uuid.UUID        `json:"id"`
	Status     AppealStatus     `json:"status"`
	ReasonCode AppealReasonCode `json:"reason_code"`
	ReasonText string           `json:"reason_text"`
	CreatedAt  time.Time        `json:"created_at"`
}

type AppealRecord struct {
	Appeal        Appeal             `json:"appeal"`
	Violation     Violation          `json:"violation"`
	Contractor    *OrgBrief          `json:"contractor"`
	Driver        *DriverBrief       `json:"driver"`
	Vehicle       *VehicleBrief      `json:"vehicle"`
	Area          *AreaBrief         `json:"cleaning_area"`
	PolygonName   *string            `json:"polygon_name"`
	Comments      []AppealCommentDTO `json:"comments"`
	Attachments   []AppealAttachment `json:"attachments"`
	LastUpdatedBy *uuid.UUID         `json:"last_updated_by"`
}

type AppealCommentDTO struct {
	AppealComment
	AuthorName string `json:"author_name"`
}
