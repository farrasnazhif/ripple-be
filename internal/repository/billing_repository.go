package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type BillingRepository interface {
	ApplySubscription(ctx context.Context, userID, role string, credits int, paypalSubscriptionID, status string) error
	CancelAutoRenewal(ctx context.Context, userID string) error
	GetByPayPalSubscriptionID(ctx context.Context, subscriptionID string) (*entity.User, error)
	ConsumeCredit(ctx context.Context, userID string) error
	AddCredits(ctx context.Context, userID string, credits int) error
}

type billingRepository struct {
	db *sql.DB
}

func NewBillingRepository(db *sql.DB) BillingRepository {
	return &billingRepository{db: db}
}

func (r *billingRepository) ApplySubscription(ctx context.Context, userID, role string, credits int, paypalSubscriptionID, status string) error {
	query := `UPDATE users
		SET role = $1, credits = $2, paypal_subscription_id = NULLIF($3, ''), subscription_status = $4,
			subscription_created_at = CASE WHEN $1 = 'pro' THEN NOW() WHEN $1 = 'free' THEN NULL ELSE subscription_created_at END,
			expired_at = CASE WHEN $1 = 'pro' THEN NOW() + INTERVAL '30 days' WHEN $1 = 'free' THEN NULL ELSE expired_at END
		WHERE id = $5`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	res, err := r.db.ExecContext(ctx, query, role, credits, paypalSubscriptionID, status, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *billingRepository) CancelAutoRenewal(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `UPDATE users SET subscription_status = 'cancelled' WHERE id = $1`, userID)
	return err
}

func (r *billingRepository) GetByPayPalSubscriptionID(ctx context.Context, subscriptionID string) (*entity.User, error) {
	query := `SELECT id, email, username, created_at, subscription_created_at, expired_at, is_active, role, credits, subscription_status, COALESCE(paypal_subscription_id, '')
		FROM users WHERE paypal_subscription_id = $1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var user entity.User
	err := r.db.QueryRowContext(ctx, query, subscriptionID).Scan(
		&user.ID, &user.Email, &user.Username, &user.CreatedAt,
		&user.SubscriptionCreatedAt, &user.ExpiredAt,
		&user.IsActive, &user.Role, &user.Credits, &user.SubscriptionStatus, &user.PayPalSubscriptionID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *billingRepository) ConsumeCredit(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `UPDATE users SET credits = credits - 1 WHERE id = $1 AND credits > 0`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return entity.ErrInsufficientCredits
	}
	return nil
}

func (r *billingRepository) AddCredits(ctx context.Context, userID string, credits int) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `UPDATE users SET credits = credits + $1 WHERE id = $2`, credits, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return entity.ErrNotFound
	}
	return nil
}
