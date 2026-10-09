package handler

import (
	"io"
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	_ "github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

type BillingHandler struct {
	billingService service.BillingService
}

func NewBillingHandler(billingService service.BillingService) *BillingHandler {
	return &BillingHandler{billingService: billingService}
}

// CreateSubscription godoc
//
//	@Summary		Create PayPal subscription
//	@Description	Create a new Pro subscription via PayPal
//	@Tags			billing
//	@Produce		json
//	@Security		BearerAuth
//	@Success		201	{object}	response.Subscription
//	@Failure		401
//	@Failure		500
//	@Router			/billing/paypal/subscriptions [post]
func (h *BillingHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	userID := helpers.UserIDFromContext(r)

	result, err := h.billingService.CreateSubscription(r.Context(), userID)
	if err != nil {
		helpers.ErrorResponse(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	helpers.SuccessResponse(w, http.StatusCreated, "Subscription created", result)
}

// CancelSubscription godoc
//
//	@Summary		Cancel PayPal subscription
//	@Description	Cancel the current Pro subscription
//	@Tags			billing
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body	request.CancelSubscription	false	"Cancellation reason"
//	@Success		200
//	@Failure		401
//	@Failure		500
//	@Router			/billing/paypal/subscriptions [delete]
func (h *BillingHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	userID := helpers.UserIDFromContext(r)

	var req request.CancelSubscription
	if err := helpers.ReadJSON(r, &req); err != nil {
		req.Reason = "User requested cancellation"
	}

	if err := h.billingService.CancelSubscription(r.Context(), userID, req.Reason); err != nil {
		helpers.ErrorResponse(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}

	helpers.SuccessResponse(w, http.StatusOK, "Subscription cancelled", nil)
}

// Webhook godoc
//
//	@Summary		PayPal webhook
//	@Description	Handle PayPal subscription webhook events
//	@Tags			billing
//	@Accept			json
//	@Produce		json
//	@Success		200
//	@Failure		400
//	@Router			/billing/paypal/webhook [post]
func (h *BillingHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}

	if err := h.billingService.HandleWebhook(r.Context(), r.Header, body); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, "webhook processing failed")
		return
	}

	helpers.SuccessResponse(w, http.StatusOK, "Webhook processed", nil)
}
