package handler

import (
	"errors"
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
	"github.com/go-chi/chi/v5"
)

type ProjectHandler struct{ service *service.ProjectService }

func NewProjectHandler(s *service.ProjectService) *ProjectHandler { return &ProjectHandler{service: s} }

// ListProjects godoc
//
//	@Summary	List owned projects
//	@Tags		projects
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{array}	entity.Project
//	@Router		/projects [get]
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.service.List(r.Context(), helpers.UserIDFromContext(r))
	if err != nil {
		projectError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "projects", projects)
}

// CreateProject godoc
//
//	@Summary	Create a project
//	@Tags		projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		request.ProjectCreate	true	"Project name and optional description"
//	@Success	201		{object}	entity.Project
//	@Router		/projects [post]
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input request.ProjectCreate
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := h.service.Create(r.Context(), helpers.UserIDFromContext(r), input.Name, input.Description)
	if err != nil {
		projectError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusCreated, "project created", project)
}

// GetProject godoc
//
//	@Summary	Get an owned project
//	@Tags		projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string	true	"Project ID"
//	@Success	200			{object}	entity.Project
//	@Router		/projects/{projectID} [get]
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "projectID")
	if !uuidPath.MatchString(id) {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid project id")
		return
	}
	project, err := h.service.Get(r.Context(), helpers.UserIDFromContext(r), id)
	if err != nil {
		projectError(w, err)
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "project", project)
}

func projectError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidProject):
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, entity.ErrNotFound):
		helpers.ErrorResponse(w, http.StatusNotFound, "project not found")
	default:
		helpers.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
	}
}

// UpdateProject godoc
//
//	@Summary	Update project title and description
//	@Tags		projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string					true	"Project ID"
//	@Param		body		body		request.ProjectCreate	true	"Updated project title and description"
//	@Success	200			{object}	entity.Project
//	@Router		/projects/{projectID} [patch]
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	var input request.ProjectCreate
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, 400, err.Error())
		return
	}
	p, err := h.service.Update(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID"), input.Name, input.Description)
	if err != nil {
		projectError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "project updated", p)
}

// DeleteProject godoc
//
//	@Summary	Permanently delete an owned project and its workflow
//	@Tags		projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		projectID	path		string	true	"Project ID"
//	@Success	200			{object}	map[string]interface{}
//	@Router		/projects/{projectID} [delete]
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !workflowIDs(w, r, "projectID") {
		return
	}
	if err := h.service.Delete(r.Context(), helpers.UserIDFromContext(r), chi.URLParam(r, "projectID")); err != nil {
		projectError(w, err)
		return
	}
	helpers.SuccessResponse(w, 200, "project deleted", nil)
}
