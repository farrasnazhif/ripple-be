package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var queryTimeout = 5 * time.Second

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id string) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, userID string) error
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	query := `INSERT INTO users (username, email, password, is_active)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	err := r.db.QueryRowContext(ctx, query, user.Username, user.Email, user.PasswordHash, user.IsActive).
		Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return entity.ErrDuplicateEmail
		case err.Error() == `pq: duplicate key value violates unique constraint "users_username_key"`:
			return entity.ErrDuplicateUsername
		default:
			return err
		}
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*entity.User, error) {
	query := `SELECT id, email, username, created_at, subscription_created_at, expired_at, is_active, role, credits, subscription_status
		FROM users WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var user entity.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.CreatedAt,
		&user.SubscriptionCreatedAt, &user.ExpiredAt,
		&user.IsActive, &user.Role, &user.Credits, &user.SubscriptionStatus,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `SELECT id, email, username, password, created_at, subscription_created_at, expired_at, is_active, role, credits, subscription_status
		FROM users WHERE email = $1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var user entity.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt,
		&user.SubscriptionCreatedAt, &user.ExpiredAt,
		&user.IsActive, &user.Role, &user.Credits, &user.SubscriptionStatus,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *entity.User) error {
	query := `UPDATE users SET username = $1, email = $2, password = $3, is_active = $4 WHERE id = $5`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, query, user.Username, user.Email, user.PasswordHash, user.IsActive, user.ID)
	return err
}

func (r *userRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM users WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
