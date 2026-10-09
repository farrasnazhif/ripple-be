package handler

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"

	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
	"github.com/go-chi/chi/v5"
)

type BudgetHandler struct{ service *service.BudgetService }

func NewBudgetHandler(s *service.BudgetService) *BudgetHandler { return &BudgetHandler{s} }

// Get godoc
//
//	@Summary	Get the project media budget, estimates and latest 100 usage entries
//	@Tags		projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string	true	"Project ID"
//	@Success	200			{object}	entity.ProjectBudget
//	@Router		/projects/{projectID}/budget [get]
func (h *BudgetHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var data *entity.ProjectBudget
	data, err := h.service.Get(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"))
	if err != nil {
		workflowError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "project generation budget", data)
}

// Set godoc
//
//	@Summary	Set the owned project's media budget cap in USD cents
//	@Tags		projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string					true	"Project ID"
//	@Param		body		body		request.ProjectBudget	true	"Budget limit"
//	@Success	200			{object}	entity.ProjectBudget
//	@Router		/projects/{projectID}/budget [put]
func (h *BudgetHandler) Set(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input request.ProjectBudget
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	if input.LimitCents == nil {
		helpers.ErrorResponse(w, 400, "limit_cents is required")
		return
	}
	if err := h.service.Set(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), *input.LimitCents); err != nil {
		workflowError(w, err)
		return
	}
	h.Get(w, r)
}
