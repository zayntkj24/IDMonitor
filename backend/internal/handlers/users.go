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

type UserHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewUserHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *UserHandler {
	return &UserHandler{DB: db, Auth: authSvc}
}

func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Use(middleware.RequirePermission(h.Auth, "users.read"))

	r.Get("/", h.ListUsers)
	r.Post("/", h.CreateUser)
	r.Get("/{id}", h.GetUser)
	r.Put("/{id}", h.UpdateUser)
	r.Delete("/{id}", h.DeleteUser)
	r.Post("/{id}/reset-password", h.ResetPassword)
	r.Post("/{id}/reset-2fa", h.Reset2FA)
	r.Post("/{id}/assign-role", h.AssignRole)
	r.Delete("/{id}/roles/{roleId}", h.RemoveRole)
	r.Get("/{id}/audit-logs", h.GetUserAuditLogs)
	return r
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page := parseIntParam(r, "page", 1)
	limit := parseIntParam(r, "limit", 25)
	offset := (page - 1) * limit
	search := r.URL.Query().Get("search")

	var users []map[string]interface{}
	var total int

	countQ := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	queryQ := `SELECT u.id, u.email, u.username, u.display_name, u.status, u.two_factor_enabled,
	            u.last_login, u.created_at,
	            COALESCE(string_agg(r.name, ','), '') as roles
	            FROM users u LEFT JOIN user_roles ur ON ur.user_id = u.id LEFT JOIN roles r ON r.id = ur.role_id
	            WHERE u.deleted_at IS NULL`

	args := []interface{}{}
	argIdx := 1

	if search != "" {
		countQ += ` AND (email ILIKE $1 OR username ILIKE $1 OR display_name ILIKE $1)`
		queryQ += ` AND (u.email ILIKE $` + itoa(argIdx) + ` OR u.username ILIKE $` + itoa(argIdx) + ` OR u.display_name ILIKE $` + itoa(argIdx) + `)`
		args = append(args, "%"+search+"%")
		argIdx++
	}

	h.DB.QueryRow(r.Context(), countQ, args...).Scan(&total)

	queryQ += ` GROUP BY u.id ORDER BY u.created_at DESC LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := h.DB.Query(r.Context(), queryQ, args...)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to list users")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, email, username, status string
		var displayName *string
		var twoFA bool
		var lastLogin *time.Time
		var createdAt time.Time
		var roles string
		rows.Scan(&id, &email, &username, &displayName, &status, &twoFA, &lastLogin, &createdAt, &roles)
		users = append(users, map[string]interface{}{
			"id": id, "email": email, "username": username, "display_name": displayName,
			"status": status, "two_factor_enabled": twoFA,
			"last_login": lastLogin, "created_at": createdAt,
			"roles": strings.Split(roles, ","),
		})
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"users": users, "total": total, "page": page, "limit": limit,
	})
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"display_name"`
		Status   string `json:"status"`
		RoleIDs  []string `json:"role_ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Email == "" || req.Username == "" || req.Password == "" {
		middleware.WriteError(w, 400, "email, username, password required")
		return
	}

	hash, err := h.Auth.HashPassword(req.Password)
	if err != nil {
		middleware.WriteError(w, 500, "hash error")
		return
	}

	if req.Status == "" { req.Status = "ACTIVE" }

	var userID string
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO users (email, username, password_hash, display_name, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		strings.ToLower(req.Email), strings.ToLower(req.Username), hash, req.Name, req.Status).Scan(&userID)
	if err != nil {
		middleware.WriteError(w, 409, "User already exists or invalid data")
		return
	}

	for _, roleID := range req.RoleIDs {
		h.DB.Exec(r.Context(), `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID)
	}

	h.Auth.CreateAuditLog(r.Context(), middleware.GetUserID(r.Context()), middleware.GetUserEmail(r.Context()),
		"USER_CREATED", "user", &userID, nil, middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 201, map[string]string{"id": userID})
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var user struct {
		ID, Email, Username, Status string
		DisplayName, LastLogin      *string
		TwoFA                       bool
		CreatedAt                   time.Time
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, email, username, display_name, status, two_factor_enabled, last_login, created_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&user.ID, &user.Email, &user.Username, &user.DisplayName, &user.Status,
			&user.TwoFA, &user.LastLogin, &user.CreatedAt)
	if err != nil {
		middleware.WriteError(w, 404, "User not found")
		return
	}

	roles, _ := h.Auth.GetUserPermissions(r.Context(), id)
	rows, _ := h.DB.Query(r.Context(),
		`SELECT r.id, r.name FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = $1`, id)
	defer rows.Close()
	var roleList []map[string]string
	for rows.Next() {
		var rid, rname string
		rows.Scan(&rid, &rname)
		roleList = append(roleList, map[string]string{"id": rid, "name": rname})
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id": user.ID, "email": user.Email, "username": user.Username,
		"display_name": user.DisplayName, "status": user.Status,
		"two_factor_enabled": user.TwoFA, "last_login": user.LastLogin,
		"created_at": user.CreatedAt, "roles": roleList, "permissions": roles,
	})
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	sets := []string{}
	args := []interface{}{}
	idx := 1
	if req.Email != "" {
		sets = append(sets, "email=$"+itoa(idx)); args = append(args, req.Email); idx++
	}
	if req.DisplayName != "" {
		sets = append(sets, "display_name=$"+itoa(idx)); args = append(args, req.DisplayName); idx++
	}
	if req.Status != "" {
		sets = append(sets, "status=$"+itoa(idx)); args = append(args, req.Status); idx++
	}
	sets = append(sets, "updated_at=NOW()")
	args = append(args, id)

	h.DB.Exec(r.Context(), `UPDATE users SET `+strings.Join(sets, ", ")+` WHERE id=$`+itoa(idx), args...)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Updated"})
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec(r.Context(), `UPDATE users SET deleted_at = NOW(), status = 'DISABLED' WHERE id = $1`, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "User deleted"})
}

func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		NewPassword string `json:"new_password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if len(req.NewPassword) < 8 {
		middleware.WriteError(w, 400, "Password min 8 chars")
		return
	}
	hash, _ := h.Auth.HashPassword(req.NewPassword)
	h.DB.Exec(r.Context(), `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`, hash, id)

	actorID := middleware.GetUserID(r.Context())
	h.Auth.CreateAuditLog(r.Context(), actorID, middleware.GetUserEmail(r.Context()),
		"ADMIN_PASSWORD_RESET", "user", &id, nil, middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]string{"message": "Password reset"})
}

func (h *UserHandler) Reset2FA(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec(r.Context(),
		`UPDATE users SET two_factor_enabled = FALSE, two_factor_secret = NULL, updated_at = NOW() WHERE id = $1`, id)
	h.DB.Exec(r.Context(), `DELETE FROM user_recovery_codes WHERE user_id = $1`, id)

	actorID := middleware.GetUserID(r.Context())
	h.Auth.CreateAuditLog(r.Context(), actorID, middleware.GetUserEmail(r.Context()),
		"ADMIN_RESET_USER_2FA", "user", &id, nil, middleware.GetClientIP(r), r.UserAgent())

	middleware.WriteJSON(w, 200, map[string]string{"message": "2FA reset"})
}

func (h *UserHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		RoleID string `json:"role_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.DB.Exec(r.Context(), `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, req.RoleID)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Role assigned"})
}

func (h *UserHandler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	roleID := chi.URLParam(r, "roleId")
	h.DB.Exec(r.Context(), `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, id, roleID)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Role removed"})
}

func (h *UserHandler) GetUserAuditLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, action, target_type, target_id, ip_address, created_at
		 FROM audit_logs WHERE actor_id = $1 ORDER BY created_at DESC LIMIT 100`, id)
	if err != nil {
		middleware.WriteError(w, 500, "Error")
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var aid, action, tgtType string
		var tgtID *string
		var ip string
		var ts time.Time
		rows.Scan(&aid, &action, &tgtType, &tgtID, &ip, &ts)
		logs = append(logs, map[string]interface{}{
			"id": aid, "action": action, "target_type": tgtType,
			"target_id": tgtID, "ip_address": ip, "created_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, logs)
}

// Roles routes
type RoleHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewRoleHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *RoleHandler {
	return &RoleHandler{DB: db, Auth: authSvc}
}

func (h *RoleHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Use(middleware.RequirePermission(h.Auth, "roles.read"))
	r.Get("/", h.ListRoles)
	r.Post("/", h.CreateRole)
	r.Put("/{id}", h.UpdateRole)
	r.Delete("/{id}", h.DeleteRole)
	r.Get("/permissions", h.ListPermissions)
	return r
}

func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, name, description, is_system FROM roles ORDER BY name`)
	defer rows.Close()
	var roles []map[string]interface{}
	for rows.Next() {
		var id, name string
		var desc *string
		var sys bool
		rows.Scan(&id, &name, &desc, &sys)
		roles = append(roles, map[string]interface{}{
			"id": id, "name": name, "description": desc, "is_system": sys,
		})
	}
	middleware.WriteJSON(w, 200, roles)
}

func (h *RoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		PermIDs     []string `json:"permission_ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var roleID string
	err := h.DB.QueryRow(r.Context(),
		`INSERT INTO roles (name, description) VALUES ($1, $2) RETURNING id`,
		req.Name, req.Description).Scan(&roleID)
	if err != nil {
		middleware.WriteError(w, 409, "Role already exists")
		return
	}

	for _, pid := range req.PermIDs {
		h.DB.Exec(r.Context(), `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roleID, pid)
	}

	middleware.WriteJSON(w, 201, map[string]string{"id": roleID})
}

func (h *RoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		PermIDs     []string `json:"permission_ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.DB.Exec(r.Context(), `UPDATE roles SET name=$1, description=$2, updated_at=NOW() WHERE id=$3`,
		req.Name, req.Description, id)

	if req.PermIDs != nil {
		h.DB.Exec(r.Context(), `DELETE FROM role_permissions WHERE role_id=$1`, id)
		for _, pid := range req.PermIDs {
			h.DB.Exec(r.Context(), `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, id, pid)
		}
	}

	middleware.WriteJSON(w, 200, map[string]string{"message": "Updated"})
}

func (h *RoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec(r.Context(), `DELETE FROM roles WHERE id=$1 AND is_system=FALSE`, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Deleted"})
}

func (h *RoleHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(), `SELECT id, resource, action, description FROM permissions ORDER BY resource, action`)
	defer rows.Close()
	var perms []map[string]interface{}
	for rows.Next() {
		var id, resource, action string
		var desc *string
		rows.Scan(&id, &resource, &action, &desc)
		perms = append(perms, map[string]interface{}{
			"id": id, "resource": resource, "action": action, "description": desc,
		})
	}
	middleware.WriteJSON(w, 200, perms)
}

func parseIntParam(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" { return def }
	var n int
	for _, c := range v {
		if c >= '0' && c <= '9' { n = n*10 + int(c-'0') }
	}
	if n == 0 { return def }
	return n
}

func itoa(n int) string {
	return strings.TrimRight(strings.TrimRight(
		func() string {
			if n == 0 { return "0" }
			s := ""
			for n > 0 { s = string(rune('0'+n%10)) + s; n /= 10 }
			return s
		}(), ""), "")
}

var _ = pgx.ErrNoRows
