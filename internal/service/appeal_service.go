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

type AttachmentInput struct {
	FileURL  string
	FileType model.AttachmentFileType
}

type AppealService struct {
	scopeRepo      *repository.ScopeRepository
	violationRepo  *repository.ViolationRepository
	appealRepo     *repository.AppealRepository
	maxAttachments int
}

func NewAppealService(
	scopeRepo *repository.ScopeRepository,
	violationRepo *repository.ViolationRepository,
	appealRepo *repository.AppealRepository,
	maxAttachments int,
) *AppealService {
	return &AppealService{
		scopeRepo:      scopeRepo,
		violationRepo:  violationRepo,
		appealRepo:     appealRepo,
		maxAttachments: maxAttachments,
	}
}

type AppealListOptions struct {
	Statuses       []model.AppealStatus
	ReasonCodes    []model.AppealReasonCode
	ViolationTypes []model.ViolationType
	ContractorIDs  []uuid.UUID
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

func (s *AppealService) List(ctx context.Context, principal model.Principal, opts AppealListOptions) ([]model.Appeal, error) {
	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return nil, err
	}

	filter := repository.AppealFilter{
		Scope:          scope,
		Statuses:       opts.Statuses,
		ReasonCodes:    opts.ReasonCodes,
		ViolationTypes: opts.ViolationTypes,
		ContractorIDs:  opts.ContractorIDs,
		DateFrom:       opts.DateFrom,
		DateTo:         opts.DateTo,
		Limit:          opts.Limit,
		Offset:         opts.Offset,
	}

	if scope.Type == model.ScopeDriver && scope.DriverID != nil {
		filter.DriverID = scope.DriverID
	}

	if scope.Type == model.ScopeTechnical {
		// Technical users focus on camera issues only.
		filter.ReasonCodes = []model.AppealReasonCode{model.AppealReasonCameraError}
	}

	appeals, err := s.appealRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	return appeals, nil
}

func (s *AppealService) Get(ctx context.Context, principal model.Principal, appealID uuid.UUID) (*model.Appeal, error) {
	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return nil, err
	}
	appeal, err := s.appealRepo.GetByID(ctx, scope, appealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return appeal, nil
}

func (s *AppealService) Create(ctx context.Context, principal model.Principal, violationID uuid.UUID, reasonCode model.AppealReasonCode, reasonText string, attachments []AttachmentInput) (*model.Appeal, error) {
	if !(principal.IsDriver() || principal.IsContractor()) {
		return nil, ErrPermissionDenied
	}

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

	if principal.IsDriver() {
		if violation.Trip == nil || violation.Trip.Driver == nil || principal.DriverID == nil || violation.Trip.Driver.ID != *principal.DriverID {
			return nil, ErrPermissionDenied
		}
	}
	if principal.IsContractor() {
		if violation.Trip == nil || violation.Trip.Ticket == nil || violation.Trip.Ticket.Contractor == nil || violation.Trip.Ticket.Contractor.ID != principal.OrgID {
			return nil, ErrPermissionDenied
		}
	}

	activeCount, err := s.appealRepo.CountActiveByViolation(ctx, violation.ID)
	if err != nil {
		return nil, err
	}
	if activeCount > 0 {
		return nil, ErrConflict
	}

	reasonText = strings.TrimSpace(reasonText)
	if len(reasonText) < 10 {
		return nil, ErrInvalidInput
	}

	if len(attachments) > s.maxAttachments {
		return nil, ErrInvalidInput
	}

	modelAttachments, err := s.buildAttachments(attachments, principal.UserID)
	if err != nil {
		return nil, err
	}

	appeal := &model.Appeal{
		ViolationID:  violation.ID,
		TripID:       violation.TripID,
		ReasonCode:   reasonCode,
		ReasonText:   reasonText,
		Status:       model.AppealStatusSubmitted,
		DriverID:     nil,
		TicketID:     nil,
		ContractorID: nil,
	}

	if violation.Trip != nil {
		if violation.Trip.Driver != nil {
			appeal.DriverID = &violation.Trip.Driver.ID
		}
		if violation.Trip.Ticket != nil {
			appeal.TicketID = &violation.Trip.Ticket.ID
			if violation.Trip.Ticket.Contractor != nil {
				id := violation.Trip.Ticket.Contractor.ID
				appeal.ContractorID = &id
			}
		}
	}

	comment := &model.AppealComment{
		AuthorID:   principal.UserID,
		AuthorRole: principal.Role,
		Message:    reasonText,
	}

	if err := s.appealRepo.CreateAppeal(ctx, appeal, modelAttachments, comment); err != nil {
		return nil, err
	}

	if err := s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
		AppealID:  appeal.ID,
		NewStatus: model.AppealStatusSubmitted,
		Note:      reasonText,
		ChangedBy: &principal.UserID,
	}); err != nil {
		return nil, err
	}

	created, err := s.appealRepo.GetByID(ctx, scope, appeal.ID)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *AppealService) AddComment(ctx context.Context, principal model.Principal, appealID uuid.UUID, message string, attachments []AttachmentInput) error {
	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return err
	}

	appeal, err := s.appealRepo.GetByID(ctx, scope, appealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	if !s.canParticipate(principal, appeal) && !principal.IsAkimat() && !principal.IsKgu() && !principal.IsToo() {
		return ErrPermissionDenied
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return ErrInvalidInput
	}

	if len(attachments) > s.maxAttachments {
		return ErrInvalidInput
	}

	modelAttachments, err := s.buildAttachments(attachments, principal.UserID)
	if err != nil {
		return err
	}

	comment := &model.AppealComment{
		AppealID:   appeal.ID,
		AuthorID:   principal.UserID,
		AuthorRole: principal.Role,
		Message:    message,
	}

	if err := s.appealRepo.AddComment(ctx, comment, modelAttachments); err != nil {
		return err
	}

	if appeal.Status == model.AppealStatusNeedInfo && (principal.IsDriver() || principal.IsContractor()) {
		oldStatus := appeal.Status
		if err := s.appealRepo.UpdateStatus(ctx, appeal.ID, model.AppealStatusUnderReview, nil); err != nil {
			return err
		}
		appeal.Status = model.AppealStatusUnderReview
		if err := s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
			AppealID:  appeal.ID,
			OldStatus: &oldStatus,
			NewStatus: appeal.Status,
			Note:      "answer received",
			ChangedBy: &principal.UserID,
		}); err != nil {
			return err
		}
	}

	return nil
}

type AppealAction string

const (
	AppealActionStartReview AppealAction = "UNDER_REVIEW"
	AppealActionNeedInfo    AppealAction = "NEED_INFO"
	AppealActionApprove     AppealAction = "APPROVE"
	AppealActionReject      AppealAction = "REJECT"
	AppealActionClose       AppealAction = "CLOSE"
)

func (s *AppealService) Act(ctx context.Context, principal model.Principal, appealID uuid.UUID, action AppealAction, message string) error {
	if !(principal.IsAkimat() || principal.IsKgu()) {
		return ErrPermissionDenied
	}

	scope, err := s.resolveScope(ctx, principal)
	if err != nil {
		return err
	}

	appeal, err := s.appealRepo.GetByID(ctx, scope, appealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	switch action {
	case AppealActionStartReview:
		if appeal.Status != model.AppealStatusSubmitted && appeal.Status != model.AppealStatusNeedInfo {
			return ErrInvalidStatus
		}
		oldStatus := appeal.Status
		if err := s.appealRepo.UpdateStatus(ctx, appeal.ID, model.AppealStatusUnderReview, nil); err != nil {
			return err
		}
		appeal.Status = model.AppealStatusUnderReview
		return s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
			AppealID:  appeal.ID,
			OldStatus: &oldStatus,
			NewStatus: appeal.Status,
			Note:      "taken into review",
			ChangedBy: &principal.UserID,
		})
	case AppealActionNeedInfo:
		if appeal.Status != model.AppealStatusUnderReview {
			return ErrInvalidStatus
		}
		if strings.TrimSpace(message) == "" {
			return ErrInvalidInput
		}
		oldStatus := appeal.Status
		if err := s.appealRepo.UpdateStatus(ctx, appeal.ID, model.AppealStatusNeedInfo, nil); err != nil {
			return err
		}
		appeal.Status = model.AppealStatusNeedInfo
		if err := s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
			AppealID:  appeal.ID,
			OldStatus: &oldStatus,
			NewStatus: appeal.Status,
			Note:      "requesting additional info",
			ChangedBy: &principal.UserID,
		}); err != nil {
			return err
		}
		return s.AddComment(ctx, principal, appeal.ID, message, nil)
	case AppealActionApprove:
		if appeal.Status != model.AppealStatusUnderReview && appeal.Status != model.AppealStatusNeedInfo {
			return ErrInvalidStatus
		}
		oldStatus := appeal.Status
		if err := s.appealRepo.UpdateStatus(ctx, appeal.ID, model.AppealStatusApproved, &principal.UserID); err != nil {
			return err
		}
		appeal.Status = model.AppealStatusApproved
		if err := s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
			AppealID:  appeal.ID,
			OldStatus: &oldStatus,
			NewStatus: appeal.Status,
			Note:      "appeal approved",
			ChangedBy: &principal.UserID,
		}); err != nil {
			return err
		}
		prevViolationStatus := appeal.Violation.Status
		if err := s.violationRepo.UpdateStatus(ctx, appeal.ViolationID, model.ViolationStatusCanceled, "canceled via appeal approval"); err != nil {
			return err
		}
		appeal.Violation.Status = model.ViolationStatusCanceled
		return s.violationRepo.LogStatusChange(ctx, &model.ViolationStatusLog{
			ViolationID: appeal.ViolationID,
			OldStatus:   &prevViolationStatus,
			NewStatus:   model.ViolationStatusCanceled,
			Note:        "canceled via appeal approval",
			ChangedBy:   &principal.UserID,
		})
	case AppealActionReject:
		if appeal.Status != model.AppealStatusUnderReview && appeal.Status != model.AppealStatusNeedInfo {
			return ErrInvalidStatus
		}
		oldStatus := appeal.Status
		if err := s.appealRepo.UpdateStatus(ctx, appeal.ID, model.AppealStatusRejected, &principal.UserID); err != nil {
			return err
		}
		appeal.Status = model.AppealStatusRejected
		if err := s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
			AppealID:  appeal.ID,
			OldStatus: &oldStatus,
			NewStatus: appeal.Status,
			Note:      "appeal rejected",
			ChangedBy: &principal.UserID,
		}); err != nil {
			return err
		}
		prevViolationStatus := appeal.Violation.Status
		if err := s.violationRepo.UpdateStatus(ctx, appeal.ViolationID, model.ViolationStatusFixed, "violation confirmed"); err != nil {
			return err
		}
		appeal.Violation.Status = model.ViolationStatusFixed
		return s.violationRepo.LogStatusChange(ctx, &model.ViolationStatusLog{
			ViolationID: appeal.ViolationID,
			OldStatus:   &prevViolationStatus,
			NewStatus:   model.ViolationStatusFixed,
			Note:        "violation confirmed via appeal rejection",
			ChangedBy:   &principal.UserID,
		})
	case AppealActionClose:
		if appeal.Status != model.AppealStatusApproved && appeal.Status != model.AppealStatusRejected {
			return ErrInvalidStatus
		}
		oldStatus := appeal.Status
		if err := s.appealRepo.UpdateStatus(ctx, appeal.ID, model.AppealStatusClosed, &principal.UserID); err != nil {
			return err
		}
		appeal.Status = model.AppealStatusClosed
		return s.appealRepo.LogStatusChange(ctx, &model.AppealStatusLog{
			AppealID:  appeal.ID,
			OldStatus: &oldStatus,
			NewStatus: appeal.Status,
			Note:      "appeal closed",
			ChangedBy: &principal.UserID,
		})
	default:
		return ErrInvalidInput
	}
}

func (s *AppealService) resolveScope(ctx context.Context, principal model.Principal) (model.Scope, error) {
	scope, err := s.scopeRepo.ResolveScope(ctx, principal)
	if err != nil {
		if errors.Is(err, repository.ErrScopeUnsupported) {
			return model.Scope{}, ErrPermissionDenied
		}
		return model.Scope{}, err
	}
	return scope, nil
}

func (s *AppealService) canParticipate(principal model.Principal, appeal *model.Appeal) bool {
	switch {
	case principal.IsDriver():
		return appeal.DriverID != nil && principal.DriverID != nil && *appeal.DriverID == *principal.DriverID
	case principal.IsContractor():
		return appeal.ContractorID != nil && *appeal.ContractorID == principal.OrgID
	default:
		return false
	}
}

func (s *AppealService) buildAttachments(inputs []AttachmentInput, uploader uuid.UUID) ([]model.AppealAttachment, error) {
	attachments := make([]model.AppealAttachment, 0, len(inputs))
	for _, att := range inputs {
		if strings.TrimSpace(att.FileURL) == "" {
			return nil, ErrInvalidInput
		}
		attachments = append(attachments, model.AppealAttachment{
			FileURL:    att.FileURL,
			FileType:   att.FileType,
			UploadedBy: uploader,
		})
	}
	return attachments, nil
}
