package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	UserEmailKey contextKey = "user_email"
	SessionIDKey contextKey = "session_id"
	PermissionsKey contextKey = "permissions"
)

type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old attempts
	attempts := rl.attempts[key]
	valid := make([]time.Time, 0)
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		return false
	}

	rl.attempts[key] = append(valid, now)
	return true
}

// Auth middleware validates JWT session
func Auth(authSvc *auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				http.Error(w, `{"error":"unauthorized","message":"Missing authentication token"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authSvc.ValidateToken(token)
			if err != nil {
				http.Error(w, `{"error":"unauthorized","message":"Invalid token"}`, http.StatusUnauthorized)
				return
			}

			// Check session exists and not revoked
			var revokedAt *time.Time
			err = authSvc.DB.QueryRow(r.Context(),
				`SELECT revoked_at FROM sessions WHERE id = $1 AND user_id = $2`,
				claims.SessionID, claims.UserID).Scan(&revokedAt)
			if err != nil {
				http.Error(w, `{"error":"unauthorized","message":"Session not found"}`, http.StatusUnauthorized)
				return
			}
			if revokedAt != nil {
				http.Error(w, `{"error":"unauthorized","message":"Session revoked"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)
			ctx = context.WithValue(ctx, SessionIDKey, claims.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission checks if the user has the specified permission
func RequirePermission(authSvc *auth.AuthService, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			has, err := authSvc.HasPermission(r.Context(), userID, permission)
			if err != nil || !has {
				http.Error(w, `{"error":"forbidden","message":"Insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit applies rate limiting
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				key = strings.Split(fwd, ",")[0]
			}

			if !limiter.Allow(key) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":"rate_limited","message":"Too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestID adds a unique request ID
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), "request_id", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger logs requests
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(wrapped, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

// CORS middleware
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

func GetUserEmail(ctx context.Context) string {
	if v, ok := ctx.Value(UserEmailKey).(string); ok {
		return v
	}
	return ""
}

func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(SessionIDKey).(string); ok {
		return v
	}
	return ""
}

func GetClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	return r.RemoteAddr
}

func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if cookie, err := r.Cookie("session_token"); err == nil {
		return cookie.Value
	}
	return ""
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Require2FAVerified ensures the session has completed 2FA if required
func Require2FAVerified(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			sessionID := GetSessionID(r.Context())

			// Check if user has 2FA enabled
			var twoFAEnabled bool
			err := db.QueryRow(r.Context(),
				`SELECT two_factor_enabled FROM users WHERE id = $1`, userID).Scan(&twoFAEnabled)
			if err == nil && twoFAEnabled {
				// Check session 2FA status
				var verified bool
				err = db.QueryRow(r.Context(),
					`SELECT is_2fa_verified FROM sessions WHERE id = $1`, sessionID).Scan(&verified)
				if err != nil || !verified {
					http.Error(w, `{"error":"2fa_required","message":"Two-factor authentication required"}`, http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
