package model

import (
	"time"

	"github.com/google/uuid"
)

type Trip struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TicketID        *uuid.UUID `gorm:"type:uuid"`
	DriverID        *uuid.UUID `gorm:"type:uuid"`
	VehicleID       *uuid.UUID `gorm:"type:uuid"`
	PolygonID       *uuid.UUID `gorm:"type:uuid"`
	Status          string     `gorm:"type:trip_status"`
	EntryAt         *time.Time `gorm:"column:entry_at"`
	ViolationReason *string    `gorm:"column:violation_reason"`
	Ticket          *Ticket    `gorm:"foreignKey:TicketID"`
	Driver          *Driver    `gorm:"foreignKey:DriverID"`
	Vehicle         *Vehicle   `gorm:"foreignKey:VehicleID"`
	Polygon         *Polygon   `gorm:"foreignKey:PolygonID"`
}

func (Trip) TableName() string {
	return "trips"
}

type Ticket struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey"`
	ContractorID   uuid.UUID     `gorm:"type:uuid"`
	CleaningAreaID uuid.UUID     `gorm:"type:uuid"`
	Status         string        `gorm:"type:ticket_status"`
	PlannedStartAt *time.Time    `gorm:"column:planned_start_at"`
	PlannedEndAt   *time.Time    `gorm:"column:planned_end_at"`
	Contractor     *Organization `gorm:"foreignKey:ContractorID"`
	CleaningArea   *CleaningArea `gorm:"foreignKey:CleaningAreaID"`
}

func (Ticket) TableName() string {
	return "tickets"
}

type Organization struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"type:varchar(255)"`
}

func (Organization) TableName() string {
	return "organizations"
}

type Driver struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	FullName string    `gorm:"type:varchar(255)"`
	Phone    string    `gorm:"type:varchar(32)"`
}

func (Driver) TableName() string {
	return "drivers"
}

type Vehicle struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	PlateNumber string    `gorm:"type:varchar(32)"`
	Brand       string    `gorm:"type:varchar(64)"`
	Model       string    `gorm:"type:varchar(64)"`
}

func (Vehicle) TableName() string {
	return "vehicles"
}

type CleaningArea struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"type:text"`
}

func (CleaningArea) TableName() string {
	return "cleaning_areas"
}

type Polygon struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"type:text"`
}

func (Polygon) TableName() string {
	return "polygons"
}
