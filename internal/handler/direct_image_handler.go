package handler

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/go-chi/chi/v5"
)

// GenerateImage godoc
//
//	@Summary	Generate a final image directly from the saved brief summary
//	@Tags		projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string				true	"Project ID"
//	@Param		body		body		request.DirectImage	true	"Saved brief version and idempotency key"
//	@Success	202			{object}	entity.FinalOutput
//	@Router		/projects/{projectID}/images [post]
func (h *ReviewHandler) GenerateImage(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input request.DirectImage
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if !uuidPath.MatchString(input.RequestKey) {
		helpers.ErrorResponse(w, 400, "invalid request key")
		return
	}
	var f *entity.FinalOutput
	f, err := h.refinements.GenerateImage(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), input.RequestKey, input.ExpectedVersion)
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 202, "final image generation saved", f)
}
