package handler

import (
	"errors"
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/farrasnazhif/ripple-be/internal/service"
	"github.com/go-chi/chi/v5"
)

type WorkflowHandler struct {
	shots *service.StoryboardService
	jobs  *service.GenerationService
}

func NewWorkflowHandler(s *service.StoryboardService, g *service.GenerationService) *WorkflowHandler {
	return &WorkflowHandler{s, g}
}
func workflowIDs(w http.ResponseWriter, r *http.Request, names ...string) bool {
	for _, name := range names {
		if !uuidPath.MatchString(chi.URLParam(r, name)) {
			helpers.ErrorResponse(w, 400, "invalid "+name)
			return false
		}
	}
	return true
}
func (h *WorkflowHandler) ListShots(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	data, err := h.shots.List(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "storyboard", data)
}
func (h *WorkflowHandler) CreateShots(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input struct {
		Summary string                  `json:"summary"`
		Shots   []entity.StoryboardShot `json:"shots"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if err := h.shots.Create(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), input.Summary, input.Shots); err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 201, "storyboard saved", nil)
}
func (h *WorkflowHandler) Submit(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "shotID") {
		return
	}
	var input struct {
		Kind       string `json:"kind"`
		Prompt     string `json:"prompt"`
		RequestKey string `json:"request_key"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if !uuidPath.MatchString(input.RequestKey) {
		helpers.ErrorResponse(w, 400, "invalid request key")
		return
	}
	j, err := h.jobs.Submit(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "shotID"), input.Kind, input.RequestKey, input.Prompt)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 202, "generation request saved", j)
}
func (h *WorkflowHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "jobID") {
		return
	}
	j, err := h.jobs.Get(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "jobID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "generation status", j)
}
func (h *WorkflowHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	data, err := h.jobs.List(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "generation history", data)
}
func workflowError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrBudgetRate):
		helpers.ErrorResponse(w, 503, err.Error())
	case errors.Is(err, repository.ErrBudgetExceeded), errors.Is(err, repository.ErrBudgetUnset):
		helpers.ErrorResponse(w, 402, err.Error())
	case errors.Is(err, repository.ErrBudgetLimit):
		helpers.ErrorResponse(w, 400, err.Error())
	case errors.Is(err, entity.ErrNotFound):
		helpers.ErrorResponse(w, 404, "project, shot or generation not found")
	case errors.Is(err, service.ErrFinalRegeneration), errors.Is(err, service.ErrInvalidDirectImage), errors.Is(err, service.ErrInvalidFinalRefinement), errors.Is(err, service.ErrFinalReference), errors.Is(err, service.ErrInvalidReview), errors.Is(err, service.ErrFinalNotReady), errors.Is(err, service.ErrInvalidStoryboard), errors.Is(err, service.ErrInvalidGeneration), errors.Is(err, service.ErrNoBriefText):
		helpers.ErrorResponse(w, 400, err.Error())
	case errors.Is(err, repository.ErrWorkflowConflict), errors.Is(err, repository.ErrGenerationConflict):
		helpers.ErrorResponse(w, 409, err.Error())
	case errors.Is(err, service.ErrVideoAssembly), errors.Is(err, service.ErrStorageUnavailable), errors.Is(err, service.ErrGenerationUnavailable):
		helpers.ErrorResponse(w, 503, err.Error())
	default:
		helpers.ErrorResponse(w, 502, "Could not complete the request. Reload the project before retrying.")
	}
}
