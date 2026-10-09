package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/lib/pq"
)

var queryTimeout = 5 * time.Second

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, id string) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
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
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			switch pgErr.Constraint {
			case "users_email_key":
				return entity.ErrDuplicateEmail
			case "users_username_key":
				return entity.ErrDuplicateUsername
			}
		}
		return err
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*entity.User, error) {
	query := `SELECT id, email, username, created_at, is_active
		FROM users WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var user entity.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Username, &user.CreatedAt, &user.IsActive,
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
	query := `SELECT id, email, username, password, created_at, is_active
		FROM users WHERE email = $1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var user entity.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}
