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
//	@Description	Create a user account and send OTP for verification
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body	request.Register	true	"Registration payload"
//	@Success		201
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

	if err := h.authService.Register(r.Context(), req); err != nil {
		handleServiceError(w, err)
		return
	}

	helpers.SuccessResponse(w, http.StatusCreated, "User registered successfully. OTP has been sent to your email.", map[string]string{"email": req.Email})
}

// SendOTP godoc
//
//	@Summary		Send OTP for registration
//	@Description	Send OTP code to email for verification
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body	request.SendOTP	true	"Email payload"
//	@Success		200
//	@Failure		400
//	@Router			/auth/otp/send [post]
func (h *AuthHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req request.SendOTP
	if err := helpers.ReadJSON(r, &req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := helpers.Validate.Struct(req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	msg, err := h.authService.SendOTP(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	helpers.SuccessResponse(w, http.StatusOK, msg, nil)
}

// VerifyOTP godoc
//
//	@Summary		Verify OTP and activate account
//	@Description	Verify OTP code and activate user account, returns JWT token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		request.VerifyOTP				true	"OTP verification payload"
//	@Success		200		{object}	response.UserWithToken
//	@Failure		400
//	@Router			/auth/otp/verify [post]
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req request.VerifyOTP
	if err := helpers.ReadJSON(r, &req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := helpers.Validate.Struct(req); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.authService.VerifyOTP(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	helpers.SuccessResponse(w, http.StatusOK, "OTP verified successfully. Account activated", result)
}

// Login godoc
//
//	@Summary		Login user
//	@Description	Authenticate with email and password, returns JWT token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		request.Login					true	"Login payload"
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
	case errors.Is(err, entity.ErrNotFound):
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid otp")
	case errors.Is(err, entity.ErrOTPExpired):
		helpers.ErrorResponse(w, http.StatusBadRequest, "otp expired")
	case errors.Is(err, entity.ErrInvalidPassword):
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid credentials")
	case errors.Is(err, entity.ErrAccountInactive):
		helpers.ErrorResponse(w, http.StatusBadRequest, "account not activated")
	default:
		helpers.ErrorResponse(w, http.StatusInternalServerError, "internal server error")
	}
}
