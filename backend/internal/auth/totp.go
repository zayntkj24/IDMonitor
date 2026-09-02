package auth

import (
	"context"
	"fmt"

	"github.com/pquerna/otp/totp"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TOTPService struct {
	DB *pgxpool.Pool
}

func NewTOTPService(db *pgxpool.Pool) *TOTPService {
	return &TOTPService{DB: db}
}

func (s *TOTPService) GenerateSecret(accountName string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "IDmonitor",
		AccountName: accountName,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to generate TOTP secret: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

func (s *TOTPService) ValidateCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

func (s *TOTPService) Enable2FA(ctx context.Context, userID, secret string) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE users SET two_factor_enabled = TRUE, two_factor_secret = $1, updated_at = NOW() WHERE id = $2`,
		secret, userID)
	return err
}

func (s *TOTPService) Disable2FA(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE users SET two_factor_enabled = FALSE, two_factor_secret = NULL, updated_at = NOW() WHERE id = $1`,
		userID)
	if err != nil {
		return err
	}
	// Delete recovery codes
	s.DB.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID)
	return nil
}

func (s *TOTPService) GetSecret(ctx context.Context, userID string) (string, bool, error) {
	var secret string
	var enabled bool
	err := s.DB.QueryRow(ctx,
		`SELECT COALESCE(two_factor_secret, ''), two_factor_enabled FROM users WHERE id = $1`,
		userID).Scan(&secret, &enabled)
	if err != nil {
		return "", false, err
	}
	return secret, enabled, nil
}
