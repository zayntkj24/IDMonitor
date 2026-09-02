package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/idmonitor/backend/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type TokenClaims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	SessionID string `json:"session_id"`
	TwoFAOK   bool   `json:"two_fa_ok"`
	jwt.RegisteredClaims
}

type AuthService struct {
	DB  *pgxpool.Pool
	Cfg *config.Config
}

func NewAuthService(db *pgxpool.Pool, cfg *config.Config) *AuthService {
	return &AuthService{DB: db, Cfg: cfg}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *AuthService) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *AuthService) GenerateToken(userID, email, sessionID string, twoFAOK bool) (string, error) {
	claims := TokenClaims{
		UserID:    userID,
		Email:     email,
		SessionID: sessionID,
		TwoFAOK:   twoFAOK,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.Cfg.SessionMaxAge)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "idmonitor",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.Cfg.JWTSecret))
}

func (s *AuthService) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.Cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

func (s *AuthService) CreateSession(ctx context.Context, userID, ipAddress, userAgent string, twoFAVerified bool) (string, string, error) {
	sessionID := uuid.New().String()
	token, err := s.GenerateToken(userID, "", sessionID, twoFAVerified)
	if err != nil {
		return "", "", err
	}

	_, err = s.DB.Exec(ctx,
		`INSERT INTO sessions (id, user_id, token, ip_address, user_agent, is_2fa_verified, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sessionID, userID, token, ipAddress, userAgent, twoFAVerified,
		time.Now().Add(s.Cfg.SessionMaxAge),
	)
	if err != nil {
		return "", "", err
	}

	return sessionID, token, nil
}

func (s *AuthService) GetSession(ctx context.Context, token string) (pgx.Row, error) {
	return s.DB.QueryRow(ctx,
		`SELECT id, user_id, is_2fa_verified, expires_at, revoked_at
		 FROM sessions WHERE token = $1`, token), nil
}

func (s *AuthService) RevokeSession(ctx context.Context, sessionID string) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE id = $1`, sessionID)
	return err
}

func (s *AuthService) RevokeAllUserSessions(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (s *AuthService) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT DISTINCT p.resource || '.' || p.action
		 FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 JOIN user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, nil
}

func (s *AuthService) HasPermission(ctx context.Context, userID, permission string) (bool, error) {
	var count int
	err := s.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 JOIN user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1 AND p.resource || '.' || p.action = $2`,
		userID, permission).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *AuthService) LogLoginAttempt(ctx context.Context, userID, email, ipAddress, userAgent string, success bool, reason string) {
	userIDVal := sql.NullString{String: userID, Valid: userID != ""}
	s.DB.Exec(ctx,
		`INSERT INTO login_history (user_id, email, success, ip_address, user_agent, failure_reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userIDVal, email, success, ipAddress, userAgent, reason)
}

func (s *AuthService) CreateAuditLog(ctx context.Context, actorID, actorEmail, action, targetType string, targetID *string, details map[string]interface{}, ip, ua string) {
	s.DB.Exec(ctx,
		`INSERT INTO audit_logs (actor_id, actor_email, action, target_type, target_id, details, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sql.NullString{String: actorID, Valid: actorID != ""},
		actorEmail, action, targetType,
		sql.NullString{String: func() string { if targetID != nil { return *targetID }; return "" }(), Valid: targetID != nil},
		details, ip, ua)
}

// GenerateRecoveryCodes generates 8 recovery codes and returns them in plain text (to show once)
func (s *AuthService) GenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	// Delete old codes
	s.DB.Exec(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID)

	codes := make([]string, 8)
	for i := 0; i < 8; i++ {
		code := generateRandomCode(8)
		codes[i] = code
		hash := sha256.Sum256([]byte(code))
		hashStr := fmt.Sprintf("%x", hash)
		s.DB.Exec(ctx,
			`INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2)`,
			userID, hashStr)
	}
	return codes, nil
}

// UseRecoveryCode checks and marks a recovery code as used
func (s *AuthService) UseRecoveryCode(ctx context.Context, userID, code string) (bool, error) {
	hash := sha256.Sum256([]byte(code))
	hashStr := fmt.Sprintf("%x", hash)

	var id string
	err := s.DB.QueryRow(ctx,
		`SELECT id FROM user_recovery_codes WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`,
		userID, hashStr).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	s.DB.Exec(ctx, `UPDATE user_recovery_codes SET used_at = NOW() WHERE id = $1`, id)
	return true, nil
}

func generateRandomCode(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return strings.ToUpper(base64.RawURLEncoding.EncodeToString(b)[:length])
}
