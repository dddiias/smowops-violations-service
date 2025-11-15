package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"violation-service/internal/http/middleware"
	"violation-service/internal/model"
	"violation-service/internal/service"
)

type Handler struct {
	violationService *service.ViolationService
	appealService    *service.AppealService
	log              zerolog.Logger
}

func NewHandler(
	violationService *service.ViolationService,
	appealService *service.AppealService,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		violationService: violationService,
		appealService:    appealService,
		log:              log,
	}
}

func (h *Handler) listViolations(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	opts, err := parseViolationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	records, err := h.violationService.List(c.Request.Context(), principal, opts)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(records))
}

func (h *Handler) getViolation(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid violation id"))
		return
	}

	details, err := h.violationService.GetDetails(c.Request.Context(), principal, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(details))
}

func (h *Handler) createViolation(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	var req struct {
		TripID      string `json:"trip_id" binding:"required"`
		Type        string `json:"type" binding:"required"`
		DetectedBy  string `json:"detected_by" binding:"required"`
		Severity    string `json:"severity" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	tripID, err := uuid.Parse(strings.TrimSpace(req.TripID))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid trip_id"))
		return
	}

	vType := model.ViolationType(strings.ToUpper(strings.TrimSpace(req.Type)))
	detected := model.ViolationDetectedBy(strings.ToUpper(strings.TrimSpace(req.DetectedBy)))
	severity := model.ViolationSeverity(strings.ToUpper(strings.TrimSpace(req.Severity)))

	input := service.CreateViolationInput{
		TripID:      tripID,
		Type:        vType,
		DetectedBy:  detected,
		Severity:    severity,
		Description: req.Description,
	}

	record, err := h.violationService.CreateManual(c.Request.Context(), principal, input)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, successResponse(record))
}

func (h *Handler) updateViolationStatus(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid violation id"))
		return
	}

	var req struct {
		Status      string `json:"status" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	status := model.ViolationStatus(strings.ToUpper(strings.TrimSpace(req.Status)))

	if err := h.violationService.UpdateStatus(c.Request.Context(), principal, id, status, req.Description); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"status": "updated"}))
}

func (h *Handler) listAppeals(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	opts, err := parseAppealQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	appeals, err := h.appealService.List(c.Request.Context(), principal, opts)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(appeals))
}

func (h *Handler) getAppeal(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid appeal id"))
		return
	}

	appeal, err := h.appealService.Get(c.Request.Context(), principal, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(appeal))
}

func (h *Handler) createAppeal(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	violationID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid violation id"))
		return
	}

	var req struct {
		ReasonCode  string              `json:"reason_code" binding:"required"`
		ReasonText  string              `json:"reason_text" binding:"required"`
		Attachments []AttachmentPayload `json:"attachments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	reasonCode := model.AppealReasonCode(strings.ToUpper(strings.TrimSpace(req.ReasonCode)))
	attachments := convertAttachmentPayloads(req.Attachments)

	appeal, err := h.appealService.Create(c.Request.Context(), principal, violationID, reasonCode, req.ReasonText, attachments)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, successResponse(appeal))
}

func (h *Handler) addAppealComment(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	appealID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid appeal id"))
		return
	}

	var req struct {
		Message     string              `json:"message" binding:"required"`
		Attachments []AttachmentPayload `json:"attachments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	attachments := convertAttachmentPayloads(req.Attachments)

	if err := h.appealService.AddComment(c.Request.Context(), principal, appealID, req.Message, attachments); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"status": "commented"}))
}

func (h *Handler) actOnAppeal(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("principal missing"))
		return
	}

	appealID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid appeal id"))
		return
	}

	var req struct {
		Action  string `json:"action" binding:"required"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	action := service.AppealAction(strings.ToUpper(strings.TrimSpace(req.Action)))

	if err := h.appealService.Act(c.Request.Context(), principal, appealID, action, req.Message); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"status": "updated"}))
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch err {
	case service.ErrPermissionDenied:
		c.JSON(http.StatusForbidden, errorResponse(err.Error()))
	case service.ErrInvalidInput:
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
	case service.ErrNotFound:
		c.JSON(http.StatusNotFound, errorResponse(err.Error()))
	case service.ErrConflict:
		c.JSON(http.StatusConflict, errorResponse(err.Error()))
	case service.ErrInvalidStatus:
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
	default:
		h.log.Error().Err(err).Msg("handler error")
		c.JSON(http.StatusInternalServerError, errorResponse("internal error"))
	}
}

func parseViolationQuery(c *gin.Context) (service.ListViolationsOptions, error) {
	var opts service.ListViolationsOptions

	if statusParam := c.Query("status"); statusParam != "" {
		for _, val := range splitCSV(statusParam) {
			opts.Statuses = append(opts.Statuses, model.ViolationStatus(strings.ToUpper(val)))
		}
	}
	if typeParam := c.Query("type"); typeParam != "" {
		for _, val := range splitCSV(typeParam) {
			opts.Types = append(opts.Types, model.ViolationType(strings.ToUpper(val)))
		}
	}
	if severityParam := c.Query("severity"); severityParam != "" {
		for _, val := range splitCSV(severityParam) {
			opts.Severities = append(opts.Severities, model.ViolationSeverity(strings.ToUpper(val)))
		}
	}
	if detectedParam := c.Query("detected_by"); detectedParam != "" {
		for _, val := range splitCSV(detectedParam) {
			opts.DetectedBy = append(opts.DetectedBy, model.ViolationDetectedBy(strings.ToUpper(val)))
		}
	}
	if contractorParam := c.Query("contractor_id"); contractorParam != "" {
		for _, val := range splitCSV(contractorParam) {
			id, err := uuid.Parse(val)
			if err != nil {
				return opts, err
			}
			opts.ContractorIDs = append(opts.ContractorIDs, id)
		}
	}
	if driverID := strings.TrimSpace(c.Query("driver_id")); driverID != "" {
		id, err := uuid.Parse(driverID)
		if err != nil {
			return opts, err
		}
		opts.DriverID = &id
	}
	if ticketID := strings.TrimSpace(c.Query("ticket_id")); ticketID != "" {
		id, err := uuid.Parse(ticketID)
		if err != nil {
			return opts, err
		}
		opts.TicketID = &id
	}
	if areaID := strings.TrimSpace(c.Query("cleaning_area_id")); areaID != "" {
		id, err := uuid.Parse(areaID)
		if err != nil {
			return opts, err
		}
		opts.CleaningAreaID = &id
	}
	if dateFrom := strings.TrimSpace(c.Query("date_from")); dateFrom != "" {
		ts, err := time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			return opts, err
		}
		opts.DateFrom = &ts
	}
	if dateTo := strings.TrimSpace(c.Query("date_to")); dateTo != "" {
		ts, err := time.Parse(time.RFC3339, dateTo)
		if err != nil {
			return opts, err
		}
		opts.DateTo = &ts
	}
	if limit := strings.TrimSpace(c.Query("limit")); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil {
			opts.Limit = v
		}
	}
	if offset := strings.TrimSpace(c.Query("offset")); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil {
			opts.Offset = v
		}
	}

	opts.Search = strings.TrimSpace(c.Query("search"))

	return opts, nil
}

func parseAppealQuery(c *gin.Context) (service.AppealListOptions, error) {
	var opts service.AppealListOptions

	if statusParam := c.Query("status"); statusParam != "" {
		for _, val := range splitCSV(statusParam) {
			opts.Statuses = append(opts.Statuses, model.AppealStatus(strings.ToUpper(val)))
		}
	}
	if reasonParam := c.Query("reason_code"); reasonParam != "" {
		for _, val := range splitCSV(reasonParam) {
			opts.ReasonCodes = append(opts.ReasonCodes, model.AppealReasonCode(strings.ToUpper(val)))
		}
	}
	if typeParam := c.Query("violation_type"); typeParam != "" {
		for _, val := range splitCSV(typeParam) {
			opts.ViolationTypes = append(opts.ViolationTypes, model.ViolationType(strings.ToUpper(val)))
		}
	}
	if contractorParam := c.Query("contractor_id"); contractorParam != "" {
		for _, val := range splitCSV(contractorParam) {
			id, err := uuid.Parse(val)
			if err != nil {
				return opts, err
			}
			opts.ContractorIDs = append(opts.ContractorIDs, id)
		}
	}
	if dateFrom := strings.TrimSpace(c.Query("date_from")); dateFrom != "" {
		ts, err := time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			return opts, err
		}
		opts.DateFrom = &ts
	}
	if dateTo := strings.TrimSpace(c.Query("date_to")); dateTo != "" {
		ts, err := time.Parse(time.RFC3339, dateTo)
		if err != nil {
			return opts, err
		}
		opts.DateTo = &ts
	}
	if limit := strings.TrimSpace(c.Query("limit")); limit != "" {
		if v, err := strconv.Atoi(limit); err == nil {
			opts.Limit = v
		}
	}
	if offset := strings.TrimSpace(c.Query("offset")); offset != "" {
		if v, err := strconv.Atoi(offset); err == nil {
			opts.Offset = v
		}
	}
	return opts, nil
}

type AttachmentPayload struct {
	FileURL  string `json:"file_url" binding:"required"`
	FileType string `json:"file_type" binding:"required"`
}

func convertAttachmentPayloads(payloads []AttachmentPayload) []service.AttachmentInput {
	result := make([]service.AttachmentInput, 0, len(payloads))
	for _, p := range payloads {
		result = append(result, service.AttachmentInput{
			FileURL:  p.FileURL,
			FileType: model.AttachmentFileType(strings.ToUpper(strings.TrimSpace(p.FileType))),
		})
	}
	return result
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

type responseEnvelope struct {
	Data interface{} `json:"data"`
}

func successResponse(data interface{}) responseEnvelope {
	return responseEnvelope{Data: data}
}

func errorResponse(msg string) gin.H {
	return gin.H{"error": msg}
}
