package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"violation-service/internal/model"
)

type AppealRepository struct {
	db *gorm.DB
}

func NewAppealRepository(db *gorm.DB) *AppealRepository {
	return &AppealRepository{db: db}
}

type AppealFilter struct {
	Scope          model.Scope
	Statuses       []model.AppealStatus
	ReasonCodes    []model.AppealReasonCode
	ViolationTypes []model.ViolationType
	ContractorIDs  []uuid.UUID
	DriverID       *uuid.UUID
	ViolationID    *uuid.UUID
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

func (r *AppealRepository) List(ctx context.Context, filter AppealFilter) ([]model.Appeal, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Appeal{}).
		Joins("JOIN violations v ON v.id = violation_appeals.violation_id").
		Joins("JOIN trips t ON t.id = v.trip_id").
		Joins("LEFT JOIN tickets tk ON tk.id = t.ticket_id")

	query = applyScopeFilter(query, filter.Scope)

	if len(filter.Statuses) > 0 {
		query = query.Where("violation_appeals.status IN ?", filter.Statuses)
	}
	if len(filter.ReasonCodes) > 0 {
		query = query.Where("violation_appeals.reason_code IN ?", filter.ReasonCodes)
	}
	if len(filter.ViolationTypes) > 0 {
		query = query.Where("v.type IN ?", filter.ViolationTypes)
	}
	if len(filter.ContractorIDs) > 0 {
		query = query.Where("tk.contractor_id IN ?", filter.ContractorIDs)
	}
	if filter.DriverID != nil {
		query = query.Where("t.driver_id = ?", *filter.DriverID)
	}
	if filter.ViolationID != nil {
		query = query.Where("violation_appeals.violation_id = ?", *filter.ViolationID)
	}
	if filter.DateFrom != nil {
		query = query.Where("violation_appeals.created_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		query = query.Where("violation_appeals.created_at <= ?", *filter.DateTo)
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	} else {
		query = query.Limit(200)
	}

	var appeals []model.Appeal
	if err := query.Order("violation_appeals.created_at DESC").
		Preload("Violation").
		Preload("Violation.Trip").
		Preload("Violation.Trip.Ticket").
		Preload("Violation.Trip.Ticket.Contractor").
		Preload("Violation.Trip.Ticket.CleaningArea").
		Preload("Violation.Trip.Driver").
		Preload("Violation.Trip.Vehicle").
		Preload("Violation.Trip.Polygon").
		Preload("Driver").
		Find(&appeals).Error; err != nil {
		return nil, err
	}

	return appeals, nil
}

func (r *AppealRepository) GetByID(ctx context.Context, scope model.Scope, id uuid.UUID) (*model.Appeal, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Appeal{}).
		Joins("JOIN violations v ON v.id = violation_appeals.violation_id").
		Joins("JOIN trips t ON t.id = v.trip_id").
		Joins("LEFT JOIN tickets tk ON tk.id = t.ticket_id").
		Where("violation_appeals.id = ?", id)

	query = applyScopeFilter(query, scope)

	var appeal model.Appeal
	if err := query.
		Preload("Violation").
		Preload("Violation.Trip").
		Preload("Violation.Trip.Ticket").
		Preload("Violation.Trip.Ticket.Contractor").
		Preload("Violation.Trip.Ticket.CleaningArea").
		Preload("Violation.Trip.Driver").
		Preload("Violation.Trip.Vehicle").
		Preload("Violation.Trip.Polygon").
		Preload("Attachments").
		Preload("Comments").
		Preload("Driver").
		First(&appeal).Error; err != nil {
		return nil, err
	}

	return &appeal, nil
}

func (r *AppealRepository) ListByViolationID(ctx context.Context, scope model.Scope, violationID uuid.UUID) ([]model.Appeal, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Appeal{}).
		Joins("JOIN violations v ON v.id = violation_appeals.violation_id").
		Joins("JOIN trips t ON t.id = v.trip_id").
		Joins("LEFT JOIN tickets tk ON tk.id = t.ticket_id").
		Where("violation_appeals.violation_id = ?", violationID)

	query = applyScopeFilter(query, scope)

	var appeals []model.Appeal
	if err := query.
		Order("violation_appeals.created_at ASC").
		Preload("Attachments").
		Preload("Comments").
		Preload("Driver").
		Find(&appeals).Error; err != nil {
		return nil, err
	}

	return appeals, nil
}

func (r *AppealRepository) CreateAppeal(ctx context.Context, appeal *model.Appeal, attachments []model.AppealAttachment, comment *model.AppealComment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(appeal).Error; err != nil {
			return err
		}

		if len(attachments) > 0 {
			for i := range attachments {
				attachments[i].AppealID = appeal.ID
			}
			if err := tx.Create(&attachments).Error; err != nil {
				return err
			}
		}

		if comment != nil {
			comment.AppealID = appeal.ID
			if err := tx.Create(comment).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *AppealRepository) AddComment(ctx context.Context, comment *model.AppealComment, attachments []model.AppealAttachment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		if len(attachments) > 0 {
			for i := range attachments {
				attachments[i].AppealID = comment.AppealID
			}
			if err := tx.Create(&attachments).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AppealRepository) UpdateStatus(ctx context.Context, appealID uuid.UUID, status model.AppealStatus, resolvedBy *uuid.UUID) error {
	data := map[string]interface{}{
		"status": status,
	}
	if status == model.AppealStatusApproved || status == model.AppealStatusRejected || status == model.AppealStatusClosed {
		data["resolved_at"] = time.Now()
		data["resolved_by"] = resolvedBy
	} else {
		data["resolved_at"] = gorm.Expr("NULL")
		data["resolved_by"] = gorm.Expr("NULL")
	}
	return r.db.WithContext(ctx).
		Model(&model.Appeal{}).
		Where("id = ?", appealID).
		Updates(data).Error
}

func (r *AppealRepository) CountActiveByViolation(ctx context.Context, violationID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Appeal{}).
		Where("violation_id = ? AND status IN ?", violationID, []model.AppealStatus{
			model.AppealStatusSubmitted,
			model.AppealStatusUnderReview,
			model.AppealStatusNeedInfo,
		}).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

type AppealSummary struct {
	LastAppeal *model.Appeal
	HasActive  bool
	HasCamera  bool
}

func (r *AppealRepository) SummariesByViolationIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]AppealSummary, error) {
	result := make(map[uuid.UUID]AppealSummary)
	if len(ids) == 0 {
		return result, nil
	}

	var appeals []model.Appeal
	if err := r.db.WithContext(ctx).
		Model(&model.Appeal{}).
		Where("violation_id IN ?", ids).
		Order("violation_id, created_at DESC").
		Find(&appeals).Error; err != nil {
		return nil, err
	}

	for _, appeal := range appeals {
		entry := result[appeal.ViolationID]
		if entry.LastAppeal == nil {
			copy := appeal
			entry.LastAppeal = &copy
		}
		if appeal.Status == model.AppealStatusSubmitted ||
			appeal.Status == model.AppealStatusUnderReview ||
			appeal.Status == model.AppealStatusNeedInfo {
			entry.HasActive = true
		}
		if appeal.ReasonCode == model.AppealReasonCameraError {
			entry.HasCamera = true
		}
		result[appeal.ViolationID] = entry
	}

	return result, nil
}

func (r *AppealRepository) LogStatusChange(ctx context.Context, logEntry *model.AppealStatusLog) error {
	return r.db.WithContext(ctx).Create(logEntry).Error
}
