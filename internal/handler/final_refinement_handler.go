package handler

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/go-chi/chi/v5"
)

// RefineFinal godoc
//
//	@Summary	Refine a saved final image or video using it as a reference
//	@Tags		projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string					true	"Project ID"
//	@Param		finalID		path		string					true	"Source final output ID"
//	@Param		body		body		request.FinalRefinement	true	"Refinement instructions and idempotency key"
//	@Success	202			{object}	entity.FinalOutput
//	@Router		/projects/{projectID}/finals/{finalID}/refinements [post]
func (h *ReviewHandler) RefineFinal(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "finalID") {
		return
	}
	var input request.FinalRefinement
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if !uuidPath.MatchString(input.RequestKey) {
		helpers.ErrorResponse(w, 400, "invalid request key")
		return
	}
	var f *entity.FinalOutput
	f, err := h.refinements.Refine(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "finalID"), input.RequestKey, input.ImageIndex, input.Prompt, input.Reason)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 202, "final refinement saved; review before accepting", f)
}

// GetFinal godoc
//
//	@Summary	Get a final output and reconcile its generation status
//	@Tags		projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string	true	"Project ID"
//	@Param		finalID		path		string	true	"Final output ID"
//	@Success	200			{object}	entity.FinalOutput
//	@Router		/projects/{projectID}/finals/{finalID} [get]
func (h *ReviewHandler) GetFinal(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "finalID") {
		return
	}
	f, err := h.refinements.Get(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "finalID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "final output status", f)
}

// AcceptFinal godoc
//
//	@Summary	Accept a completed final output version
//	@Tags		projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string	true	"Project ID"
//	@Param		finalID		path		string	true	"Final output ID"
//	@Success	200			{object}	entity.FinalOutput
//	@Router		/projects/{projectID}/finals/{finalID}/accept [post]
func (h *ReviewHandler) AcceptFinal(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "finalID") {
		return
	}
	f, err := h.refinements.Accept(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "finalID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "final output accepted", f)
}
