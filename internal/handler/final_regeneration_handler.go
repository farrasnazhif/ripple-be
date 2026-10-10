package handler

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/go-chi/chi/v5"
)

// RegenerateFinal godoc
//
// @Summary Generate another iteration using the saved prompt and original reference
// @Tags projects
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param projectID path string true "Project ID"
// @Param finalID path string true "Saved final output ID"
// @Param body body request.FinalRegeneration true "Image index and idempotency key"
// @Success 202 {object} entity.FinalOutput
// @Router /projects/{projectID}/finals/{finalID}/regenerations [post]
func (h *ReviewHandler) RegenerateFinal(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID", "finalID") {
		return
	}
	var input request.FinalRegeneration
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if !uuidPath.MatchString(input.RequestKey) {
		helpers.ErrorResponse(w, 400, "invalid request key")
		return
	}
	var f *entity.FinalOutput
	f, err := h.refinements.Regenerate(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), chi.URLParam(r, "finalID"), input.RequestKey, input.ImageIndex)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 202, "new iteration saved; review before accepting", f)
}
