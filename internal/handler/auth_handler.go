package handler

import (
	"errors"
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	_ "github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register godoc
//
//	@Summary		Register a new user
//	@Description	Create an active account and return a JWT token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		request.Register	true	"Registration payload"
//	@Success		201		{object}	response.UserWithToken
//	@Failure		400
//	@Router			/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req request.Register
	if err := helpers.ReadJSON(r, &req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := helpers.Validate.Struct(req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.authService.Register(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	helpers.SuccessResponse(w, http.StatusCreated, "Account created successfully", result)
}

// Login godoc
//
//	@Summary		Login user
//	@Description	Authenticate with email and password, returns JWT token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		request.Login	true	"Login payload"
//	@Success		200		{object}	response.UserWithToken
//	@Failure		400
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req request.Login
	if err := helpers.ReadJSON(r, &req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := helpers.Validate.Struct(req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.authService.Login(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	helpers.SuccessResponse(w, http.StatusOK, "Login successful", result)
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, entity.ErrEmailExists), errors.Is(err, entity.ErrDuplicateEmail):
		helpers.ErrorResponse(w, http.StatusBadRequest, "email already registered")
	case errors.Is(err, entity.ErrDuplicateUsername):
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, entity.ErrPasswordTooLong):
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, entity.ErrInvalidPassword):
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid credentials")
	case errors.Is(err, entity.ErrAccountInactive):
		helpers.ErrorResponse(w, http.StatusBadRequest, "account not activated")
	default:
		helpers.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
	}
}
