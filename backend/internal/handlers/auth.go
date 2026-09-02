package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/idmonitor/backend/internal/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthHandler struct {
	DB  *pgxpool.Pool
	Auth *auth.AuthService
	TOTP *auth.TOTPService
}

func NewAuthHandler(db *pgxpool.Pool, authSvc *auth.AuthService, totpSvc *auth.TOTPService) *AuthHandler {
	return &AuthHandler{DB: db, Auth: authSvc, TOTP: totpSvc}
}

func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/login", h.Login)
	r.Post("/register", h.Register)
	r.Post("/logout", h.Logout)
	r.Post("/2fa/verify", h.Verify2FA)
	r.Post("/2fa/recovery", h.UseRecoveryCode)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(h.Auth))
		r.Post("/2fa/setup", h.Setup2FA)
		r.Post("/2fa/enable", h.Enable2FA)
		r.Post("/2fa/disable", h.Disable2FA)
		r.Post("/2fa/recovery/generate", h.GenerateRecoveryCodes)
		r.Post("/password/change", h.ChangePassword)
		r.Get("/me", h.GetMe)
		r.Get("/sessions", h.GetSessions)
		r.Delete("/sessions/{id}", h.RevokeSession)
	})
	return r
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		middleware.WriteError(w, 400, "Email and password required")
		return
	}

	var user struct {
		ID                    string
		PasswordHash          string
		Status                string
		TwoFactorEnabled      bool
		FailedLoginAttempts   int
		LockedUntil           *time.Time
	}

	err := h.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, status, two_factor_enabled, failed_login_attempts, locked_until
		 FROM users WHERE email = $1 AND deleted_at IS NULL`, req.Email).
		Scan(&user.ID, &user.PasswordHash, &user.Status, &user.TwoFactorEnabled,
			&user.FailedLoginAttempts, &user.LockedUntil)

	ip := middleware.GetClientIP(r)
	ua := r.UserAgent()

	if err == pgx.ErrNoRows {
		h.Auth.LogLoginAttempt(r.Context(), "", req.Email, ip, ua, false, "user not found")
		middleware.WriteError(w, 401, "Invalid credentials")
		return
	}
	if err != nil {
		middleware.WriteError(w, 500, "Internal error")
		return
	}

	if user.Status == "LOCKED" && user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		h.Auth.LogLoginAttempt(r.Context(), user.ID, req.Email, ip, ua, false, "account locked")
		middleware.WriteError(w, 423, "Account is locked. Try again later.")
		return
	}

	if user.Status == "DISABLED" {
		h.Auth.LogLoginAttempt(r.Context(), user.ID, req.Email, ip, ua, false, "account disabled")
		middleware.WriteError(w, 403, "Account is disabled")
		return
	}

	if !h.Auth.CheckPassword(req.Password, user.PasswordHash) {
		attempts := user.FailedLoginAttempts + 1
		var lockUntil *time.Time
		if attempts >= 5 {
			t := time.Now().Add(15 * time.Minute)
			lockUntil = &t
			h.DB.Exec(r.Context(),
				`UPDATE users SET failed_login_attempts = $1, locked_until = $2, status = 'LOCKED', updated_at = NOW() WHERE id = $3`,
				attempts, lockUntil, user.ID)
		} else {
			h.DB.Exec(r.Context(),
				`UPDATE users SET failed_login_attempts = $1, updated_at = NOW() WHERE id = $2`,
				attempts, user.ID)
		}
		h.Auth.LogLoginAttempt(r.Context(), user.ID, req.Email, ip, ua, false, "wrong password")
		middleware.WriteError(w, 401, "Invalid credentials")
		return
	}

	// Reset failed attempts on successful password check
	h.DB.Exec(r.Context(),
		`UPDATE users SET failed_login_attempts = 0, locked_until = NULL, status = 'ACTIVE', last_login = NOW(), updated_at = NOW() WHERE id = $1`,
		user.ID)

	twoFAVerified := !user.TwoFactorEnabled
	sessionID, token, err := h.Auth.CreateSession(r.Context(), user.ID, ip, ua, twoFAVerified)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to create session")
		return
	}

	h.Auth.LogLoginAttempt(r.Context(), user.ID, req.Email, ip, ua, true, "")
	h.Auth.CreateAuditLog(r.Context(), user.ID, req.Email, "LOGIN_SUCCESS", "user", &user.ID, nil, ip, ua)

	resp := map[string]interface{}{
		"token":               token,
		"session_id":          sessionID,
		"two_factor_required": user.TwoFactorEnabled && !twoFAVerified,
		"user": map[string]interface{}{
			"id":                  user.ID,
			"email":               req.Email,
			"two_factor_enabled":  user.TwoFactorEnabled,
		},
	}
	middleware.WriteJSON(w, 200, resp)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(strings.ToLower(req.Username))

	if req.Email == "" || req.Username == "" || len(req.Password) < 8 {
		middleware.WriteError(w, 400, "Valid email, username, and password (min 8 chars) required")
		return
	}

	hash, err := h.Auth.HashPassword(req.Password)
	if err != nil {
		middleware.WriteError(w, 500, "Internal error")
		return
	}

	var userID string
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO users (email, username, password_hash, display_name)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Email, req.Username, hash, req.DisplayName).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			middleware.WriteError(w, 409, "Email or username already exists")
			return
		}
		middleware.WriteError(w, 500, "Failed to create user")
		return
	}

	// Assign default VIEWER role
	h.DB.Exec(r.Context(),
		`INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'VIEWER'`,
		userID)

	ip := middleware.GetClientIP(r)
	ua := r.UserAgent()
	h.Auth.CreateAuditLog(r.Context(), userID, req.Email, "USER_CREATED", "user", &userID, nil, ip, ua)

	middleware.WriteJSON(w, 201, map[string]interface{}{
		"id":       userID,
		"email":    req.Email,
		"username": req.Username,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID := middleware.GetSessionID(r.Context())
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())

	h.Auth.RevokeSession(r.Context(), sessionID)

	ip := middleware.GetClientIP(r)
	ua := r.UserAgent()
	h.Auth.CreateAuditLog(r.Context(), userID, email, "LOGOUT", "session", &sessionID, nil, ip, ua)

	middleware.WriteJSON(w, 200, map[string]string{"message": "Logged out"})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var user struct {
		ID               string
		Email            string
		Username         string
		DisplayName      *string
		Status           string
		TwoFactorEnabled bool
		CreatedAt        time.Time
	}

	err := h.DB.QueryRow(r.Context(),
		`SELECT id, email, username, display_name, status, two_factor_enabled, created_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).
		Scan(&user.ID, &user.Email, &user.Username, &user.DisplayName, &user.Status,
			&user.TwoFactorEnabled, &user.CreatedAt)
	if err != nil {
		middleware.WriteError(w, 404, "User not found")
		return
	}

	perms, _ := h.Auth.GetUserPermissions(r.Context(), userID)

	// Get roles
	rows, _ := h.DB.Query(r.Context(),
		`SELECT r.name FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = $1`, userID)
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		roles = append(roles, name)
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id":                  user.ID,
		"email":               user.Email,
		"username":            user.Username,
		"display_name":        user.DisplayName,
		"status":              user.Status,
		"two_factor_enabled":  user.TwoFactorEnabled,
		"roles":               roles,
		"permissions":         perms,
		"created_at":          user.CreatedAt,
	})
}

// 2FA Setup
type Setup2FAResponse struct {
	Secret string `json:"secret"`
	QRCode string `json:"qr_code_url"`
}

func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())

	// Check if already enabled
	var enabled bool
	h.DB.QueryRow(r.Context(), `SELECT two_factor_enabled FROM users WHERE id = $1`, userID).Scan(&enabled)
	if enabled {
		middleware.WriteError(w, 400, "2FA is already enabled")
		return
	}

	secret, qrURL, err := h.TOTP.GenerateSecret(email)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to generate TOTP secret")
		return
	}

	// Store secret temporarily (not yet enabled)
	h.DB.Exec(r.Context(),
		`UPDATE users SET two_factor_secret = $1, updated_at = NOW() WHERE id = $2`, secret, userID)

	middleware.WriteJSON(w, 200, Setup2FAResponse{
		Secret: secret,
		QRCode: qrURL,
	})
}

func (h *AuthHandler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	var secret string
	h.DB.QueryRow(r.Context(),
		`SELECT two_factor_secret FROM users WHERE id = $1`, userID).Scan(&secret)

	if secret == "" {
		middleware.WriteError(w, 400, "Please run 2FA setup first")
		return
	}

	if !h.TOTP.ValidateCode(secret, req.Code) {
		h.Auth.CreateAuditLog(r.Context(), userID, email, "2FA_FAILED", "user", &userID, nil,
			middleware.GetClientIP(r), r.UserAgent())
		middleware.WriteError(w, 400, "Invalid TOTP code")
		return
	}

	if err := h.TOTP.Enable2FA(r.Context(), userID, secret); err != nil {
		middleware.WriteError(w, 500, "Failed to enable 2FA")
		return
	}

	// Generate recovery codes
	codes, _ := h.Auth.GenerateRecoveryCodes(r.Context(), userID)

	h.Auth.CreateAuditLog(r.Context(), userID, email, "2FA_ENABLED", "user", &userID, nil,
		middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"message":        "2FA enabled successfully",
		"recovery_codes": codes,
	})
}

func (h *AuthHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	// Verify password
	var passHash string
	h.DB.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&passHash)
	if !h.Auth.CheckPassword(req.Password, passHash) {
		middleware.WriteError(w, 401, "Invalid password")
		return
	}

	// Verify TOTP
	secret, _, _ := h.TOTP.GetSecret(r.Context(), userID)
	if !h.TOTP.ValidateCode(secret, req.Code) {
		middleware.WriteError(w, 400, "Invalid TOTP code")
		return
	}

	h.TOTP.Disable2FA(r.Context(), userID)

	h.Auth.CreateAuditLog(r.Context(), userID, email, "2FA_DISABLED", "user", &userID, nil,
		middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]string{"message": "2FA disabled"})
}

func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionToken string `json:"session_token"`
		Code         string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	// Validate the session token (pre-auth token)
	claims, err := h.Auth.ValidateToken(req.SessionToken)
	if err != nil {
		middleware.WriteError(w, 401, "Invalid session token")
		return
	}

	var secret string
	h.DB.QueryRow(r.Context(),
		`SELECT u.two_factor_secret FROM users u JOIN sessions s ON s.user_id = u.id
		 WHERE s.id = $1`, claims.SessionID).Scan(&secret)

	if !h.TOTP.ValidateCode(secret, req.Code) {
		h.Auth.CreateAuditLog(r.Context(), claims.UserID, claims.Email, "2FA_FAILED", "user", &claims.UserID, nil,
			middleware.GetClientIP(r), r.UserAgent())
		middleware.WriteError(w, 400, "Invalid TOTP code")
		return
	}

	// Mark session as 2FA verified
	h.DB.Exec(r.Context(),
		`UPDATE sessions SET is_2fa_verified = TRUE WHERE id = $1`, claims.SessionID)

	h.Auth.CreateAuditLog(r.Context(), claims.UserID, claims.Email, "2FA_VERIFIED", "user", &claims.UserID, nil,
		middleware.GetClientIP(r), r.UserAgent())

	// Generate new token with 2FA verified
	newToken, err := h.Auth.GenerateToken(claims.UserID, claims.Email, claims.SessionID, true)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to generate token")
		return
	}

	h.DB.Exec(r.Context(),
		`UPDATE sessions SET token = $1 WHERE id = $2`, newToken, claims.SessionID)

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"token":      newToken,
		"session_id": claims.SessionID,
		"verified":   true,
	})
}

func (h *AuthHandler) UseRecoveryCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionToken string `json:"session_token"`
		Code         string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	claims, err := h.Auth.ValidateToken(req.SessionToken)
	if err != nil {
		middleware.WriteError(w, 401, "Invalid session token")
		return
	}

	ok, err := h.Auth.UseRecoveryCode(r.Context(), claims.UserID, req.Code)
	if err != nil || !ok {
		h.Auth.CreateAuditLog(r.Context(), claims.UserID, claims.Email, "2FA_RECOVERY_FAILED", "user", &claims.UserID, nil,
			middleware.GetClientIP(r), r.UserAgent())
		middleware.WriteError(w, 400, "Invalid or used recovery code")
		return
	}

	// Mark session as verified
	h.DB.Exec(r.Context(), `UPDATE sessions SET is_2fa_verified = TRUE WHERE id = $1`, claims.SessionID)

	newToken, _ := h.Auth.GenerateToken(claims.UserID, claims.Email, claims.SessionID, true)
	h.DB.Exec(r.Context(), `UPDATE sessions SET token = $1 WHERE id = $2`, newToken, claims.SessionID)

	h.Auth.CreateAuditLog(r.Context(), claims.UserID, claims.Email, "2FA_RECOVERY_USED", "user", &claims.UserID, nil,
		middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"token":      newToken,
		"session_id": claims.SessionID,
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	if len(req.NewPassword) < 8 {
		middleware.WriteError(w, 400, "Password must be at least 8 characters")
		return
	}

	var passHash string
	h.DB.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&passHash)
	if !h.Auth.CheckPassword(req.CurrentPassword, passHash) {
		middleware.WriteError(w, 401, "Invalid current password")
		return
	}

	newHash, err := h.Auth.HashPassword(req.NewPassword)
	if err != nil {
		middleware.WriteError(w, 500, "Internal error")
		return
	}

	h.DB.Exec(r.Context(), `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`, newHash, userID)

	// Revoke all sessions except current
	h.Auth.RevokeAllUserSessions(r.Context(), userID)

	h.Auth.CreateAuditLog(r.Context(), userID, email, "PASSWORD_CHANGED", "user", &userID, nil,
		middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]string{"message": "Password changed. Please log in again."})
}

func (h *AuthHandler) GenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())

	codes, err := h.Auth.GenerateRecoveryCodes(r.Context(), userID)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to generate codes")
		return
	}

	h.Auth.CreateAuditLog(r.Context(), userID, email, "RECOVERY_CODES_REGENERATED", "user", &userID, nil,
		middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"recovery_codes": codes,
		"message":        "Save these codes. They will not be shown again.",
	})
}

func (h *AuthHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	currentSessionID := middleware.GetSessionID(r.Context())

	rows, err := h.DB.Query(r.Context(),
		`SELECT id, ip_address, user_agent, is_2fa_verified, created_at, revoked_at
		 FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`, userID)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to fetch sessions")
		return
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, ip, ua string
		var verified bool
		var createdAt time.Time
		var revokedAt *time.Time
		rows.Scan(&id, &ip, &ua, &verified, &createdAt, &revokedAt)
		sessions = append(sessions, map[string]interface{}{
			"id":               id,
			"ip_address":       ip,
			"user_agent":       ua,
			"2fa_verified":     verified,
			"current":          id == currentSessionID,
			"created_at":       createdAt,
			"revoked_at":       revokedAt,
			"is_active":        revokedAt == nil,
		})
	}

	middleware.WriteJSON(w, 200, sessions)
}

func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessionID := chi.URLParam(r, "id")

	h.DB.Exec(r.Context(),
		`UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND user_id = $2`,
		sessionID, userID)

	middleware.WriteJSON(w, 200, map[string]string{"message": "Session revoked"})
}

var _ = strings.TrimSpace
