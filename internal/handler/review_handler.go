package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
	"github.com/go-chi/chi/v5"
)

type ReviewAI interface {
	DraftPlan(context.Context, string, string) ([]entity.StoryboardShot, error)
	RefineSummary(context.Context, string, string) (string, error)
}
type ReviewHandler struct {
	review *service.ReviewService
	ai     ReviewAI
	finals *service.FinalOutputService
}

func NewReviewHandler(review *service.ReviewService, ai ReviewAI, finals *service.FinalOutputService) *ReviewHandler {
	return &ReviewHandler{review, ai, finals}
}
func (h *ReviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	v, err := h.review.Get(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "project review", v)
}
func (h *ReviewHandler) Edit(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input entity.BriefEdit
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	for _, id := range input.RemoveAttachments {
		if !uuidPath.MatchString(id) {
			helpers.ErrorResponse(w, 400, "invalid attachment id")
			return
		}
	}
	if err := h.review.Edit(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), input); err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "new brief version saved", nil)
}
func (h *ReviewHandler) SetType(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input struct {
		Kind string `json:"kind"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if err := h.review.SetType(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), input.Kind); err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "output type saved", nil)
}
func (h *ReviewHandler) Link(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "shotID") {
		return
	}
	var input struct {
		Keys []string `json:"keys"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if err := h.review.Link(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "shotID"), input.Keys); err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "requirements linked", nil)
}
func (h *ReviewHandler) Decide(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "shotID") {
		return
	}
	var input struct {
		Job    string `json:"job_id"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if input.Job != "" && !uuidPath.MatchString(input.Job) {
		helpers.ErrorResponse(w, 400, "invalid job id")
		return
	}
	if err := h.review.Decide(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "shotID"), input.Job, input.Action, input.Reason); err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "decision saved", nil)
}
func (h *ReviewHandler) Draft(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	state, err := h.review.Get(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	var input struct {
		Summary      string `json:"summary"`
		Instructions string `json:"instructions"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if len(input.Summary) > 100000 || strings.TrimSpace(input.Summary) == "" || len(input.Instructions) > 2000 {
		helpers.ErrorResponse(w, 400, "provide a summary and valid adjustments")
		return
	}
	if h.ai == nil {
		helpers.ErrorResponse(w, 503, "ChatGPT API is not configured")
		return
	}
	if strings.HasSuffix(r.URL.Path, "summary/refine") {
		text, err := h.ai.RefineSummary(r.Context(), input.Summary, input.Instructions)
		if err != nil {
			workflowError(w, err)
			return
		}
		helpers.SuccessResponse(w, 200, "review refined summary", map[string]string{"summary": text})
		return
	}
	if state.OutputType != "image" && state.OutputType != "video" {
		helpers.ErrorResponse(w, 400, "choose image or video first")
		return
	}
	shots, err := h.ai.DraftPlan(r.Context(), input.Summary, state.OutputType)
	if err != nil {
		workflowError(w, err)
		return
	}
	if len(shots) < 1 || len(shots) > 50 {
		helpers.ErrorResponse(w, 502, "invalid plan draft; write the plan manually")
		return
	}
	for _, shot := range shots {
		if strings.TrimSpace(shot.Title) == "" || len(shot.Title) > 200 || strings.TrimSpace(shot.Prompt) == "" || len(shot.Prompt) > 10000 {
			helpers.ErrorResponse(w, 502, "invalid plan draft; write the plan manually")
			return
		}
	}
	helpers.SuccessResponse(w, 200, "review plan before saving", shots)
}
func (h *ReviewHandler) Final(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input struct {
		Kind string `json:"kind"`
	}
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	v, err := h.finals.Create(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), input.Kind)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 201, "final output saved", v)
}
