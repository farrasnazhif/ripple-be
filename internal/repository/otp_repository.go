package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type OTPRepository interface {
	Create(ctx context.Context, email string, purpose entity.OTPPurpose, expiry time.Duration) (*entity.OTPCode, error)
	Verify(ctx context.Context, email, code string, purpose entity.OTPPurpose) (*entity.OTPCode, error)
	IsVerified(ctx context.Context, email string, purpose entity.OTPPurpose) (bool, error)
	Delete(ctx context.Context, id string) error
	CleanupExpired(ctx context.Context) error
}

type otpRepository struct {
	db *sql.DB
}

func NewOTPRepository(db *sql.DB) OTPRepository {
	return &otpRepository{db: db}
}

func (r *otpRepository) Create(ctx context.Context, email string, purpose entity.OTPPurpose, expiry time.Duration) (*entity.OTPCode, error) {
	code, err := generateOTP()
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO otp_codes (email, code, purpose, expires_at)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	otp := &entity.OTPCode{
		Email:     email,
		Code:      code,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(expiry),
	}

	err = r.db.QueryRowContext(ctx, query, email, code, purpose, otp.ExpiresAt).
		Scan(&otp.ID, &otp.CreatedAt)
	if err != nil {
		return nil, err
	}
	return otp, nil
}

func (r *otpRepository) Verify(ctx context.Context, email, code string, purpose entity.OTPPurpose) (*entity.OTPCode, error) {
	query := `SELECT id, email, code, purpose, expires_at, created_at, verified_at
		FROM otp_codes WHERE email = $1 AND code = $2 AND purpose = $3
		ORDER BY created_at DESC LIMIT 1`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var otp entity.OTPCode
	var verifiedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, email, code, purpose).Scan(
		&otp.ID, &otp.Email, &otp.Code, &otp.Purpose,
		&otp.ExpiresAt, &otp.CreatedAt, &verifiedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	if verifiedAt.Valid {
		otp.VerifiedAt = &verifiedAt.Time
	}

	if !otp.ExpiresAt.After(time.Now()) {
		return nil, entity.ErrOTPExpired
	}

	now := time.Now()
	_, err = r.db.ExecContext(ctx, `UPDATE otp_codes SET verified_at = $1 WHERE id = $2`, now, otp.ID)
	if err != nil {
		return nil, err
	}
	otp.VerifiedAt = &now
	return &otp, nil
}

func (r *otpRepository) IsVerified(ctx context.Context, email string, purpose entity.OTPPurpose) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM otp_codes WHERE email = $1 AND purpose = $2 AND verified_at IS NOT NULL AND expires_at > $3)`

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var verified bool
	err := r.db.QueryRowContext(ctx, query, email, purpose, time.Now()).Scan(&verified)
	return verified, err
}

func (r *otpRepository) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `DELETE FROM otp_codes WHERE id = $1`, id)
	return err
}

func (r *otpRepository) CleanupExpired(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `DELETE FROM otp_codes WHERE expires_at < $1`, time.Now())
	return err
}

func generateOTP() (string, error) {
	const digits = "0123456789"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = digits[b[i]%10]
	}
	return string(b), nil
}
