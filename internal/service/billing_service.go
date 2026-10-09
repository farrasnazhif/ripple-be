package service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/paypal"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

const (
	freeCredits = 1
	proCredits  = 40
)

type BillingService interface {
	CreateSubscription(ctx context.Context, userID string) (*response.Subscription, error)
	CancelSubscription(ctx context.Context, userID, reason string) error
	HandleWebhook(ctx context.Context, headers http.Header, body json.RawMessage) error
}

type billingService struct {
	billingRepo repository.BillingRepository
	userRepo    repository.UserRepository
	paypal      paypal.Client
}

func NewBillingService(billingRepo repository.BillingRepository, userRepo repository.UserRepository, paypal paypal.Client) BillingService {
	return &billingService{billingRepo: billingRepo, userRepo: userRepo, paypal: paypal}
}

func (s *billingService) CreateSubscription(ctx context.Context, userID string) (*response.Subscription, error) {
	sub, err := s.paypal.CreateSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &response.Subscription{
		SubscriptionID: sub.ID,
		ApproveURL:     sub.ApproveURL,
		Status:         sub.Status,
	}, nil
}

func (s *billingService) CancelSubscription(ctx context.Context, userID, reason string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.PayPalSubscriptionID == "" {
		return nil
	}

	if err := s.paypal.CancelSubscription(ctx, user.PayPalSubscriptionID, reason); err != nil {
		return err
	}

	return s.billingRepo.ApplySubscription(ctx, userID, "free", freeCredits, "", "cancelled")
}

func (s *billingService) HandleWebhook(ctx context.Context, headers http.Header, body json.RawMessage) error {
	if err := s.paypal.VerifyWebhook(ctx, headers, body); err != nil {
		return err
	}

	var event struct {
		EventType string `json:"event_type"`
		Resource  struct {
			ID       string `json:"id"`
			CustomID string `json:"custom_id"`
			Status   string `json:"status"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	switch event.EventType {
	case "BILLING.SUBSCRIPTION.ACTIVATED":
		return s.billingRepo.ApplySubscription(ctx, event.Resource.CustomID, "pro", proCredits, event.Resource.ID, "active")

	case "BILLING.SUBSCRIPTION.CANCELLED", "BILLING.SUBSCRIPTION.EXPIRED":
		user, err := s.billingRepo.GetByPayPalSubscriptionID(ctx, event.Resource.ID)
		if err != nil {
			return err
		}
		return s.billingRepo.ApplySubscription(ctx, user.ID, "free", freeCredits, "", "cancelled")

	case "PAYMENT.SALE.COMPLETED":
		// Renewal — reset credits
		if event.Resource.CustomID != "" {
			return s.billingRepo.ApplySubscription(ctx, event.Resource.CustomID, "pro", proCredits, event.Resource.ID, "active")
		}
	}

	return nil
}
