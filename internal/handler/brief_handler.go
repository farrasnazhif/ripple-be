package handler

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/farrasnazhif/ripple-be/internal/service"
	"github.com/go-chi/chi/v5"
)

var uuidPath = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type BriefHandler struct{ service *service.BriefService }

func NewBriefHandler(s *service.BriefService) *BriefHandler { return &BriefHandler{service: s} }

func (h *BriefHandler) StorageStatus(w http.ResponseWriter, r *http.Request) {
	helpers.SuccessResponse(w, http.StatusOK, "attachment storage status", map[string]bool{"available": h.service.AttachmentStorageAvailable()})
}

func (h *BriefHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RawText string `json:"raw_text"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	brief, err := h.service.Create(r.Context(), helpers.UserIDFromContext(r), input.RawText)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusCreated, "brief draft created", brief)
}

func (h *BriefHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "briefID")
	if !uuidPath.MatchString(id) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid brief id")
		return
	}
	brief, err := h.service.Get(r.Context(), helpers.UserIDFromContext(r), id)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "brief", brief)
}

func (h *BriefHandler) DraftSummary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "briefID")
	if !uuidPath.MatchString(id) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid brief id")
		return
	}
	summary, usage, err := h.service.DraftSummary(r.Context(), helpers.UserIDFromContext(r), id)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "draft summary; confirm before use", map[string]any{"summary": summary, "usage": usage})
}

func (h *BriefHandler) ConfirmSummary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "briefID")
	if !uuidPath.MatchString(id) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid brief id")
		return
	}
	var input struct {
		Summary string `json:"summary"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.ConfirmSummary(r.Context(), helpers.UserIDFromContext(r), id, input.Summary); err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "summary confirmed", nil)
}

func (h *BriefHandler) StartAttachment(w http.ResponseWriter, r *http.Request) {
	briefID := chi.URLParam(r, "briefID")
	if !uuidPath.MatchString(briefID) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid brief id")
		return
	}
	var input struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	a, url, err := h.service.StartAttachment(r.Context(), helpers.UserIDFromContext(r), briefID, input.FileName, input.ContentType, input.SizeBytes)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusCreated, "upload URL created", map[string]any{
		"attachment": a, "upload_url": url, "upload_method": "PUT", "content_type": a.ContentType,
	})
}

func (h *BriefHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	briefID := chi.URLParam(r, "briefID")
	if !uuidPath.MatchString(briefID) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid brief id")
		return
	}
	attachments, err := h.service.ListAttachments(r.Context(), helpers.UserIDFromContext(r), briefID)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "brief attachments", attachments)
}

func (h *BriefHandler) CompleteAttachment(w http.ResponseWriter, r *http.Request) {
	briefID, attachmentID := chi.URLParam(r, "briefID"), chi.URLParam(r, "attachmentID")
	if !uuidPath.MatchString(briefID) || !uuidPath.MatchString(attachmentID) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid id")
		return
	}
	a, err := h.service.CompleteAttachment(r.Context(), helpers.UserIDFromContext(r), briefID, attachmentID)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "attachment verified", a)
}

func (h *BriefHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	briefID, attachmentID := chi.URLParam(r, "briefID"), chi.URLParam(r, "attachmentID")
	if !uuidPath.MatchString(briefID) || !uuidPath.MatchString(attachmentID) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid id")
		return
	}
	url, err := h.service.DownloadAttachment(r.Context(), helpers.UserIDFromContext(r), briefID, attachmentID)
	if err != nil {
		briefError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "temporary download URL", map[string]string{"download_url": url})
}

func briefError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrWorkflowConflict):
		helpers.ErrorResponse(w, 409, err.Error())
	case errors.Is(err, entity.ErrNotFound):
		helpers.ErrorResponse(w, http.StatusNotFound, "brief or attachment not found")
	case errors.Is(err, service.ErrInvalidAttachment):
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrEmptySummary), errors.Is(err, service.ErrSummaryInputsTooLarge):
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrStorageUnavailable):
		helpers.ErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrAIUnavailable):
		helpers.ErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrNoBriefText):
		helpers.ErrorResponse(w, http.StatusBadRequest, "add brief text or a supported image/PDF before generating a summary")
	default:
		helpers.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *BriefHandler) SetText(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "briefID") {
		return
	}
	var input struct {
		Text string `json:"raw_text"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if err := h.service.SetText(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "briefID"), input.Text); err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "brief text captured", nil)
}
