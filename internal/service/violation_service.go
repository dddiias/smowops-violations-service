package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"violation-service/internal/model"
	"violation-service/internal/repository"
)

type ViolationService struct {
	scopeRepo     *repository.ScopeRepository
	violationRepo *repository.ViolationRepository
	appealRepo    *repository.AppealRepository
}

func NewViolationService(
	scopeRepo *repository.ScopeRepository,
	violationRepo *repository.ViolationRepository,
	appealRepo *repository.AppealRepository,
) *ViolationService {
	return &ViolationService{
		scopeRepo:     scopeRepo,
		violationRepo: violationRepo,
		appealRepo:    appealRepo,
	}
}

type ListViolationsOptions struct {
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

type ViolationDetails struct {
	Record  model.ViolationRecord `json:"record"`
	Appeals []model.Appeal        `json:"appeals"`
}

func (s *ViolationService) List(ctx context.Context, principal model.Principal, opts ListViolationsOptions) ([]model.ViolationRecord, error) {
	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return nil, err
	}

	filter := repository.ViolationFilter{
		Scope:          scope,
		Statuses:       opts.Statuses,
		Types:          opts.Types,
		Severities:     opts.Severities,
		DetectedBy:     opts.DetectedBy,
		ContractorIDs:  opts.ContractorIDs,
		DriverID:       opts.DriverID,
		TicketID:       opts.TicketID,
		CleaningAreaID: opts.CleaningAreaID,
		DateFrom:       opts.DateFrom,
		DateTo:         opts.DateTo,
		Search:         opts.Search,
		Limit:          opts.Limit,
		Offset:         opts.Offset,
	}

	if scope.Type == model.ScopeDriver && scope.DriverID != nil {
		filter.DriverID = scope.DriverID
	}

	violations, err := s.violationRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(violations))
	for _, v := range violations {
		ids = append(ids, v.ID)
	}

	summaries, err := s.appealRepo.SummariesByViolationIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	requireCameraAppeal := scope.Type == model.ScopeTechnical

	records := make([]model.ViolationRecord, 0, len(violations))
	for _, v := range violations {
		summary := summaries[v.ID]
		if requireCameraAppeal && !summary.HasCamera {
			continue
		}
		record := buildViolationRecord(v, summary)
		records = append(records, record)
	}

	return records, nil
}

func (s *ViolationService) GetDetails(ctx context.Context, principal model.Principal, violationID uuid.UUID) (*ViolationDetails, error) {
	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return nil, err
	}

	violation, err := s.violationRepo.GetByID(ctx, scope, violationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	summary, err := s.appealRepo.SummariesByViolationIDs(ctx, []uuid.UUID{violation.ID})
	if err != nil {
		return nil, err
	}

	record := buildViolationRecord(*violation, summary[violation.ID])

	appeals, err := s.appealRepo.ListByViolationID(ctx, scope, violation.ID)
	if err != nil {
		return nil, err
	}

	return &ViolationDetails{
		Record:  record,
		Appeals: appeals,
	}, nil
}

type CreateViolationInput struct {
	TripID      uuid.UUID
	Type        model.ViolationType
	DetectedBy  model.ViolationDetectedBy
	Severity    model.ViolationSeverity
	Description string
}

func (s *ViolationService) CreateManual(ctx context.Context, principal model.Principal, input CreateViolationInput) (*model.ViolationRecord, error) {
	if !(principal.IsAkimat() || principal.IsKgu()) {
		return nil, ErrPermissionDenied
	}

	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return nil, err
	}

	trip, err := s.violationRepo.GetTrip(ctx, input.TripID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var contractorID *uuid.UUID
	if trip.Ticket != nil {
		contractorID = &trip.Ticket.ContractorID
	}

	if contractorID != nil && !scope.AllowsViolation(contractorID) && !principal.IsAkimat() {
		return nil, ErrPermissionDenied
	}

	violation := &model.Violation{
		TripID:      trip.ID,
		Type:        input.Type,
		DetectedBy:  input.DetectedBy,
		Severity:    input.Severity,
		Status:      model.ViolationStatusOpen,
		Description: input.Description,
	}

	if err := s.violationRepo.Create(ctx, violation); err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.Description) != "" {
		if err := s.violationRepo.UpdateTripViolationReason(ctx, trip.ID, input.Description); err != nil {
			return nil, err
		}
	}

	if err := s.violationRepo.LogStatusChange(ctx, &model.ViolationStatusLog{
		ViolationID: violation.ID,
		NewStatus:   model.ViolationStatusOpen,
		Note:        "manual creation",
		ChangedBy:   &principal.UserID,
	}); err != nil {
		return nil, err
	}

	created, err := s.violationRepo.GetByID(ctx, scope, violation.ID)
	if err != nil {
		return nil, err
	}

	record := buildViolationRecord(*created, repository.AppealSummary{})
	return &record, nil
}

func (s *ViolationService) UpdateStatus(ctx context.Context, principal model.Principal, violationID uuid.UUID, target model.ViolationStatus, description string) error {
	if !(principal.IsAkimat() || principal.IsKgu()) {
		return ErrPermissionDenied
	}

	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return err
	}

	violation, err := s.violationRepo.GetByID(ctx, scope, violationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	if violation.Status == model.ViolationStatusCanceled || violation.Status == model.ViolationStatusFixed {
		return ErrInvalidStatus
	}

	if target != model.ViolationStatusCanceled && target != model.ViolationStatusFixed {
		return ErrInvalidStatus
	}

	if err := s.violationRepo.UpdateStatus(ctx, violation.ID, target, description); err != nil {
		return err
	}

	prev := violation.Status
	if err := s.violationRepo.LogStatusChange(ctx, &model.ViolationStatusLog{
		ViolationID: violation.ID,
		OldStatus:   &prev,
		NewStatus:   target,
		Note:        description,
		ChangedBy:   &principal.UserID,
	}); err != nil {
		return err
	}

	return nil
}

func buildViolationRecord(v model.Violation, summary repository.AppealSummary) model.ViolationRecord {
	record := model.ViolationRecord{
		Violation:           v,
		TripStatus:          "",
		TripEntryAt:         nil,
		TripViolationReason: nil,
		Contractor:          nil,
		Ticket:              nil,
		Driver:              nil,
		Vehicle:             nil,
		Area:                nil,
		PolygonName:         nil,
		LastAppeal:          nil,
		HasActiveAppeal:     summary.HasActive,
	}

	if summary.LastAppeal != nil {
		copy := *summary.LastAppeal
		record.LastAppeal = &model.AppealBrief{
			ID:         copy.ID,
			Status:     copy.Status,
			ReasonCode: copy.ReasonCode,
			ReasonText: copy.ReasonText,
			CreatedAt:  copy.CreatedAt,
		}
	}

	if v.Trip != nil {
		record.TripStatus = v.Trip.Status
		record.TripEntryAt = v.Trip.EntryAt
		record.TripViolationReason = v.Trip.ViolationReason

		if v.Trip.Ticket != nil {
			ticket := v.Trip.Ticket
			record.Ticket = &model.TicketBrief{
				ID:        ticket.ID,
				Status:    ticket.Status,
				StartPlan: ticket.PlannedStartAt,
				EndPlan:   ticket.PlannedEndAt,
			}
			if ticket.Contractor != nil {
				record.Contractor = &model.OrgBrief{
					ID:   ticket.Contractor.ID,
					Name: ticket.Contractor.Name,
				}
			}
			if ticket.CleaningArea != nil {
				record.Area = &model.AreaBrief{
					ID:   ticket.CleaningArea.ID,
					Name: ticket.CleaningArea.Name,
				}
			}
		}
		if v.Trip.Driver != nil {
			record.Driver = &model.DriverBrief{
				ID:       v.Trip.Driver.ID,
				FullName: v.Trip.Driver.FullName,
				Phone:    v.Trip.Driver.Phone,
			}
		}
		if v.Trip.Vehicle != nil {
			record.Vehicle = &model.VehicleBrief{
				ID:          v.Trip.Vehicle.ID,
				PlateNumber: v.Trip.Vehicle.PlateNumber,
				Brand:       v.Trip.Vehicle.Brand,
				Model:       v.Trip.Vehicle.Model,
			}
		}
		if v.Trip.Polygon != nil {
			record.PolygonName = &v.Trip.Polygon.Name
		}
	}

	return record
}

func (s *ViolationService) resolveScope(ctx context.Context, principal model.Principal) (model.Scope, error) {
	scope, err := s.scopeRepo.ResolveScope(ctx, principal)
	if err != nil {
		if errors.Is(err, repository.ErrScopeUnsupported) {
			return model.Scope{}, ErrPermissionDenied
		}
		return model.Scope{}, err
	}
	return scope, nil
}
