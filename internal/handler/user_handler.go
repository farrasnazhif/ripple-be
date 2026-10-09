package handler

import (
	"net/http"

	_ "github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// GetProfile godoc
//
//	@Summary		Get user profile
//	@Description	Get the authenticated user's profile
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.User
//	@Failure		401
//	@Router			/users/me [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := helpers.UserIDFromContext(r)

	result, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	helpers.SuccessResponse(w, http.StatusOK, "User retrieved successfully", result)
}
