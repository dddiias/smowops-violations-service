package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"violation-service/internal/model"
)

type ViolationRepository struct {
	db *gorm.DB
}

func NewViolationRepository(db *gorm.DB) *ViolationRepository {
	return &ViolationRepository{db: db}
}

type ViolationFilter struct {
	Scope          model.Scope
	Statuses       []model.ViolationStatus
	Types          []model.ViolationType
	Severities     []model.ViolationSeverity
	DetectedBy     []model.ViolationDetectedBy
	ContractorIDs  []uuid.UUID
	DriverID       *uuid.UUID
	TicketID       *uuid.UUID
	CleaningAreaID *uuid.UUID
	DateFrom       *time.Time
	DateTo         *time.Time
	Search         string
	Limit          int
	Offset         int
}

func (r *ViolationRepository) List(ctx context.Context, filter ViolationFilter) ([]model.Violation, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Violation{}).
		Joins("JOIN trips t ON t.id = violations.trip_id").
		Joins("LEFT JOIN tickets tk ON tk.id = t.ticket_id")

	query = applyScopeFilter(query, filter.Scope)

	if len(filter.Statuses) > 0 {
		query = query.Where("violations.status IN ?", filter.Statuses)
	}
	if len(filter.Types) > 0 {
		query = query.Where("violations.type IN ?", filter.Types)
	}
	if len(filter.Severities) > 0 {
		query = query.Where("violations.severity IN ?", filter.Severities)
	}
	if len(filter.DetectedBy) > 0 {
		query = query.Where("violations.detected_by IN ?", filter.DetectedBy)
	}
	if len(filter.ContractorIDs) > 0 {
		query = query.Where("tk.contractor_id IN ?", filter.ContractorIDs)
	}
	if filter.DriverID != nil {
		query = query.Where("t.driver_id = ?", *filter.DriverID)
	}
	if filter.TicketID != nil {
		query = query.Where("tk.id = ?", *filter.TicketID)
	}
	if filter.CleaningAreaID != nil {
		query = query.Where("tk.cleaning_area_id = ?", *filter.CleaningAreaID)
	}
	if filter.DateFrom != nil {
		query = query.Where("COALESCE(t.entry_at, violations.created_at) >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		query = query.Where("COALESCE(t.entry_at, violations.created_at) <= ?", *filter.DateTo)
	}
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Joins("LEFT JOIN drivers d ON d.id = t.driver_id").
			Joins("LEFT JOIN vehicles v ON v.id = t.vehicle_id").
			Where("(d.full_name ILIKE ? OR v.plate_number ILIKE ?)", search, search)
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	} else {
		query = query.Limit(200)
	}

	var violations []model.Violation
	if err := query.
		Order("violations.created_at DESC").
		Preload("Trip").
		Preload("Trip.Ticket").
		Preload("Trip.Ticket.Contractor").
		Preload("Trip.Ticket.CleaningArea").
		Preload("Trip.Driver").
		Preload("Trip.Vehicle").
		Preload("Trip.Polygon").
		Find(&violations).Error; err != nil {
		return nil, err
	}

	return violations, nil
}

func (r *ViolationRepository) GetByID(ctx context.Context, scope model.Scope, id uuid.UUID) (*model.Violation, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Violation{}).
		Joins("JOIN trips t ON t.id = violations.trip_id").
		Joins("LEFT JOIN tickets tk ON tk.id = t.ticket_id").
		Where("violations.id = ?", id)

	query = applyScopeFilter(query, scope)

	var violation model.Violation
	err := query.
		Preload("Trip").
		Preload("Trip.Ticket").
		Preload("Trip.Ticket.Contractor").
		Preload("Trip.Ticket.CleaningArea").
		Preload("Trip.Driver").
		Preload("Trip.Vehicle").
		Preload("Trip.Polygon").
		First(&violation).Error
	if err != nil {
		return nil, err
	}
	return &violation, nil
}

func (r *ViolationRepository) Create(ctx context.Context, violation *model.Violation) error {
	return r.db.WithContext(ctx).Create(violation).Error
}

func (r *ViolationRepository) UpdateStatus(ctx context.Context, violationID uuid.UUID, status model.ViolationStatus, description string) error {
	return r.db.WithContext(ctx).
		Model(&model.Violation{}).
		Where("id = ?", violationID).
		Updates(map[string]interface{}{
			"status":      status,
			"description": description,
		}).Error
}

func (r *ViolationRepository) LogStatusChange(ctx context.Context, logEntry *model.ViolationStatusLog) error {
	return r.db.WithContext(ctx).Create(logEntry).Error
}

func (r *ViolationRepository) UpdateTripViolationReason(ctx context.Context, tripID uuid.UUID, reason string) error {
	return r.db.WithContext(ctx).
		Model(&model.Trip{}).
		Where("id = ?", tripID).
		Update("violation_reason", reason).Error
}

func (r *ViolationRepository) GetTrip(ctx context.Context, tripID uuid.UUID) (*model.Trip, error) {
	var trip model.Trip
	if err := r.db.WithContext(ctx).
		Model(&model.Trip{}).
		Preload("Ticket").
		First(&trip, "id = ?", tripID).Error; err != nil {
		return nil, err
	}
	return &trip, nil
}

func applyScopeFilter(query *gorm.DB, scope model.Scope) *gorm.DB {
	switch scope.Type {
	case model.ScopeCity:
		return query
	case model.ScopeKgu:
		if len(scope.ContractorIDs) == 0 {
			return query.Where("1=0")
		}
		return query.Where("tk.contractor_id IN ?", scope.ContractorIDs)
	case model.ScopeContractor:
		if scope.OrgID == nil {
			return query.Where("1=0")
		}
		return query.Where("tk.contractor_id = ?", *scope.OrgID)
	case model.ScopeDriver:
		if scope.DriverID == nil {
			return query.Where("1=0")
		}
		return query.Where("t.driver_id = ?", *scope.DriverID)
	case model.ScopeTechnical:
		return query.Where("violations.detected_by IN ?", []model.ViolationDetectedBy{
			model.ViolationDetectedByLpr,
			model.ViolationDetectedByVolume,
			model.ViolationDetectedBySystem,
		})
	default:
		return query.Where("1=0")
	}
}
