package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/idmonitor/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewAlertHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *AlertHandler {
	return &AlertHandler{DB: db, Auth: authSvc}
}

func (h *AlertHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/", h.ListAlerts)
	r.Get("/stats", h.AlertStats)
	r.Get("/{id}", h.GetAlert)
	r.Post("/{id}/acknowledge", h.AcknowledgeAlert)
	r.Post("/{id}/resolve", h.ResolveAlert)
	return r
}

func (h *AlertHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	severity := r.URL.Query().Get("severity")
	page := parseIntParam(r, "page", 1)
	limit := parseIntParam(r, "limit", 25)
	offset := (page - 1) * limit

	query := `SELECT id, title, description, severity, status, source, host_id, created_at, updated_at FROM alerts`
	countQ := `SELECT COUNT(*) FROM alerts`
	var args []interface{}
	var conditions []string

	if status != "" {
		conditions = append(conditions, "status=$"+itoa(len(args)+1))
		args = append(args, status)
	}
	if severity != "" {
		conditions = append(conditions, "severity=$"+itoa(len(args)+1))
		args = append(args, severity)
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
		countQ += " WHERE " + joinStrings(conditions, " AND ")
	}

	var total int
	h.DB.QueryRow(r.Context(), countQ, args...).Scan(&total)

	query += " ORDER BY created_at DESC LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		middleware.WriteError(w, 500, "Failed")
		return
	}
	defer rows.Close()

	var alerts []map[string]interface{}
	for rows.Next() {
		var id, title, sev, stat string
		var desc, source, hostID *string
		var created, updated time.Time
		rows.Scan(&id, &title, &desc, &sev, &stat, &source, &hostID, &created, &updated)
		alerts = append(alerts, map[string]interface{}{
			"id": id, "title": title, "description": desc, "severity": sev,
			"status": stat, "source": source, "host_id": hostID,
			"created_at": created, "updated_at": updated,
		})
	}
	middleware.WriteJSON(w, 200, map[string]interface{}{
		"alerts": alerts, "total": total, "page": page, "limit": limit,
	})
}

func (h *AlertHandler) AlertStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{}
	var total, open, ack, crit, high int
	h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM alerts`).Scan(&total)
	h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM alerts WHERE status='OPEN'`).Scan(&open)
	h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM alerts WHERE status='ACKNOWLEDGED'`).Scan(&ack)
	h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM alerts WHERE severity='CRITICAL' AND status IN ('OPEN','ACKNOWLEDGED')`).Scan(&crit)
	h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM alerts WHERE severity='HIGH' AND status IN ('OPEN','ACKNOWLEDGED')`).Scan(&high)
	stats["total"] = total
	stats["open"] = open
	stats["acknowledged"] = ack
	stats["critical"] = crit
	stats["high"] = high
	middleware.WriteJSON(w, 200, stats)
}

func (h *AlertHandler) GetAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var alert struct {
		ID, Title, Severity, Status string
		Desc, Source, HostID *string
		Metadata []byte
		Created, Updated time.Time
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, title, description, severity, status, source, host_id, metadata, created_at, updated_at
		 FROM alerts WHERE id=$1`, id).
		Scan(&alert.ID, &alert.Title, &alert.Desc, &alert.Severity, &alert.Status,
			&alert.Source, &alert.HostID, &alert.Metadata, &alert.Created, &alert.Updated)
	if err != nil {
		middleware.WriteError(w, 404, "Not found")
		return
	}
	var meta map[string]interface{}
	json.Unmarshal(alert.Metadata, &meta)
	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id": alert.ID, "title": alert.Title, "description": alert.Desc,
		"severity": alert.Severity, "status": alert.Status, "source": alert.Source,
		"host_id": alert.HostID, "metadata": meta,
		"created_at": alert.Created, "updated_at": alert.Updated,
	})
}

func (h *AlertHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	h.DB.Exec(r.Context(),
		`UPDATE alerts SET status='ACKNOWLEDGED', acknowledged_by=$1, acknowledged_at=NOW(), updated_at=NOW() WHERE id=$2`,
		userID, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Acknowledged"})
}

func (h *AlertHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	h.DB.Exec(r.Context(),
		`UPDATE alerts SET status='RESOLVED', resolved_by=$1, resolved_at=NOW(), updated_at=NOW() WHERE id=$2`,
		userID, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Resolved"})
}

// Incident Handler
type IncidentHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewIncidentHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *IncidentHandler {
	return &IncidentHandler{DB: db, Auth: authSvc}
}

func (h *IncidentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/", h.ListIncidents)
	r.Post("/", h.CreateIncident)
	r.Get("/{id}", h.GetIncident)
	r.Put("/{id}", h.UpdateIncident)
	r.Post("/{id}/notes", h.AddNote)
	r.Post("/{id}/alerts", h.LinkAlert)
	return r
}

func (h *IncidentHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	query := `SELECT id, title, description, severity, status, created_by, created_at, updated_at FROM incidents`
	var args []interface{}
	if status != "" {
		query += ` WHERE status=$1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT 50`

	rows, _ := h.DB.Query(r.Context(), query, args...)
	defer rows.Close()
	var incidents []map[string]interface{}
	for rows.Next() {
		var id, title, sev, stat string
		var desc *string; var createdBy *string
		var created, updated time.Time
		rows.Scan(&id, &title, &desc, &sev, &stat, &createdBy, &created, &updated)
		incidents = append(incidents, map[string]interface{}{
			"id": id, "title": title, "description": desc, "severity": sev,
			"status": stat, "created_by": createdBy, "created_at": created, "updated_at": updated,
		})
	}
	middleware.WriteJSON(w, 200, incidents)
}

func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Severity    string `json:"severity"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	userID := middleware.GetUserID(r.Context())
	var id string
	h.DB.QueryRow(r.Context(),
		`INSERT INTO incidents (title, description, severity, created_by) VALUES ($1,$2,$3,$4) RETURNING id`,
		req.Title, req.Description, req.Severity, userID).Scan(&id)
	middleware.WriteJSON(w, 201, map[string]string{"id": id})
}

func (h *IncidentHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var inc struct {
		ID, Title, Severity, Status string
		Desc *string; CreatedBy *string
		Created, Updated time.Time
	}
	h.DB.QueryRow(r.Context(),
		`SELECT id, title, description, severity, status, created_by, created_at, updated_at FROM incidents WHERE id=$1`, id).
		Scan(&inc.ID, &inc.Title, &inc.Desc, &inc.Severity, &inc.Status, &inc.CreatedBy, &inc.Created, &inc.Updated)

	// Notes
	nRows, _ := h.DB.Query(r.Context(), `SELECT id, user_id, content, created_at FROM incident_notes WHERE incident_id=$1 ORDER BY created_at`, id)
	defer nRows.Close()
	var notes []map[string]interface{}
	for nRows.Next() {
		var nid, content string; var uid *string; var ts time.Time
		nRows.Scan(&nid, &uid, &content, &ts)
		notes = append(notes, map[string]interface{}{"id": nid, "user_id": uid, "content": content, "created_at": ts})
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id": inc.ID, "title": inc.Title, "description": inc.Desc,
		"severity": inc.Severity, "status": inc.Status, "created_by": inc.CreatedBy,
		"created_at": inc.Created, "updated_at": inc.Updated, "notes": notes,
	})
}

func (h *IncidentHandler) UpdateIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Status   string `json:"status"`
		Severity string `json:"severity"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Status != "" {
		h.DB.Exec(r.Context(), `UPDATE incidents SET status=$1, updated_at=NOW() WHERE id=$2`, req.Status, id)
	}
	if req.Severity != "" {
		h.DB.Exec(r.Context(), `UPDATE incidents SET severity=$1, updated_at=NOW() WHERE id=$2`, req.Severity, id)
	}
	middleware.WriteJSON(w, 200, map[string]string{"message": "Updated"})
}

func (h *IncidentHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	userID := middleware.GetUserID(r.Context())
	h.DB.Exec(r.Context(),
		`INSERT INTO incident_notes (incident_id, user_id, content) VALUES ($1,$2,$3)`, id, userID, req.Content)
	middleware.WriteJSON(w, 201, map[string]string{"message": "Note added"})
}

func (h *IncidentHandler) LinkAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		AlertID string `json:"alert_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.DB.Exec(r.Context(),
		`INSERT INTO incident_events (incident_id, alert_id, event_type, description) VALUES ($1,$2,'ALERT_LINKED','Alert linked to incident')`,
		id, req.AlertID)
	middleware.WriteJSON(w, 201, map[string]string{"message": "Alert linked"})
}

// Security Handler
type SecurityHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewSecurityHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *SecurityHandler {
	return &SecurityHandler{DB: db, Auth: authSvc}
}

func (h *SecurityHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/events", h.ListEvents)
	r.Post("/events/{id}/acknowledge", h.AcknowledgeEvent)
	r.Get("/rules", h.ListRules)
	r.Post("/rules", h.CreateRule)
	r.Get("/fim", h.ListFIMEvents)
	r.Post("/fim/rules", h.CreateFIMRule)
	r.Get("/vulnerabilities", h.ListVulnerabilities)
	return r
}

func (h *SecurityHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	severity := r.URL.Query().Get("severity")
	query := `SELECT id, title, description, severity, source, acknowledged, created_at FROM security_events`
	var args []interface{}
	if severity != "" {
		query += ` WHERE severity=$1`
		args = append(args, severity)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, _ := h.DB.Query(r.Context(), query, args...)
	defer rows.Close()
	var events []map[string]interface{}
	for rows.Next() {
		var id, title, sev string
		var desc, source *string; var ack bool; var ts time.Time
		rows.Scan(&id, &title, &desc, &sev, &source, &ack, &ts)
		events = append(events, map[string]interface{}{
			"id": id, "title": title, "description": desc, "severity": sev,
			"source": source, "acknowledged": ack, "created_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, events)
}

func (h *SecurityHandler) AcknowledgeEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	h.DB.Exec(r.Context(),
		`UPDATE security_events SET acknowledged=TRUE, acknowledged_by=$1, acknowledged_at=NOW() WHERE id=$2`, userID, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Acknowledged"})
}

func (h *SecurityHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, name, description, category, severity, enabled FROM security_rules ORDER BY name`)
	defer rows.Close()
	var rules []map[string]interface{}
	for rows.Next() {
		var id, name, sev string; var desc, cat *string; var enabled bool
		rows.Scan(&id, &name, &desc, &cat, &sev, &enabled)
		rules = append(rules, map[string]interface{}{
			"id": id, "name": name, "description": desc, "category": cat,
			"severity": sev, "enabled": enabled,
		})
	}
	middleware.WriteJSON(w, 200, rules)
}

func (h *SecurityHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Category    string                 `json:"category"`
		Condition   map[string]interface{} `json:"condition_json"`
		Severity    string                 `json:"severity"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	condJSON, _ := json.Marshal(req.Condition)
	var id string
	h.DB.QueryRow(r.Context(),
		`INSERT INTO security_rules (name, description, category, condition_json, severity) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		req.Name, req.Description, req.Category, condJSON, req.Severity).Scan(&id)
	middleware.WriteJSON(w, 201, map[string]string{"id": id})
}

func (h *SecurityHandler) ListFIMEvents(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, host_id, file_path, event_type, old_hash, new_hash, detected_at
		 FROM fim_events ORDER BY detected_at DESC LIMIT 100`)
	defer rows.Close()
	var events []map[string]interface{}
	for rows.Next() {
		var id, fpath, etype string; var hostID, oldH, newH *string; var ts time.Time
		rows.Scan(&id, &hostID, &fpath, &etype, &oldH, &newH, &ts)
		events = append(events, map[string]interface{}{
			"id": id, "host_id": hostID, "file_path": fpath, "event_type": etype,
			"old_hash": oldH, "new_hash": newH, "detected_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, events)
}

func (h *SecurityHandler) CreateFIMRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HostID   string   `json:"host_id"`
		Path     string   `json:"path"`
		Recursive bool    `json:"recursive"`
		Exclude  []string `json:"exclude_patterns"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	var id string
	h.DB.QueryRow(r.Context(),
		`INSERT INTO fim_rules (host_id, path, recursive, exclude_patterns) VALUES ($1,$2,$3,$4) RETURNING id`,
		req.HostID, req.Path, req.Recursive, req.Exclude).Scan(&id)
	middleware.WriteJSON(w, 201, map[string]string{"id": id})
}

func (h *SecurityHandler) ListVulnerabilities(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT vf.id, v.cve_id, v.title, v.severity, v.cvss_score, vf.status, vf.found_at
		 FROM vulnerability_findings vf JOIN vulnerabilities v ON v.id = vf.vulnerability_id
		 ORDER BY vf.found_at DESC LIMIT 100`)
	defer rows.Close()
	var vulns []map[string]interface{}
	for rows.Next() {
		var id, title, sev, stat string; var cve *string; var cvss *float64; var ts time.Time
		rows.Scan(&id, &cve, &title, &sev, &cvss, &stat, &ts)
		vulns = append(vulns, map[string]interface{}{
			"id": id, "cve_id": cve, "title": title, "severity": sev,
			"cvss_score": cvss, "status": stat, "found_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, vulns)
}

// Audit Handler
type AuditHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewAuditHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *AuditHandler {
	return &AuditHandler{DB: db, Auth: authSvc}
}

func (h *AuditHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Use(middleware.RequirePermission(h.Auth, "audit.read"))
	r.Get("/", h.ListLogs)
	return r
}

func (h *AuditHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	actorID := r.URL.Query().Get("actor_id")
	page := parseIntParam(r, "page", 1)
	limit := parseIntParam(r, "limit", 50)
	offset := (page - 1) * limit

	query := `SELECT id, actor_id, actor_email, action, target_type, target_id, ip_address, created_at FROM audit_logs`
	var args []interface{}
	var conds []string
	if action != "" {
		conds = append(conds, "action=$"+itoa(len(args)+1))
		args = append(args, action)
	}
	if actorID != "" {
		conds = append(conds, "actor_id=$"+itoa(len(args)+1))
		args = append(args, actorID)
	}
	if len(conds) > 0 {
		query += " WHERE " + joinStrings(conds, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, _ := h.DB.Query(r.Context(), query, args...)
	defer rows.Close()
	var logs []map[string]interface{}
	for rows.Next() {
		var id, email, act string; var actor, tgtType, tgtID, ip *string; var ts time.Time
		rows.Scan(&id, &actor, &email, &act, &tgtType, &tgtID, &ip, &ts)
		logs = append(logs, map[string]interface{}{
			"id": id, "actor_id": actor, "actor_email": email, "action": act,
			"target_type": tgtType, "target_id": tgtID, "ip_address": ip, "created_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, logs)
}

// Health Handler
type HealthHandler struct {
	DB *pgxpool.Pool
}

func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{DB: db}
}

func (h *HealthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Health)
	r.Get("/ready", h.Ready)
	return r
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, 200, map[string]interface{}{
		"status": "ok", "service": "IDmonitor", "version": "1.0.0",
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	dbOk := true
	err := h.DB.Ping(r.Context())
	if err != nil { dbOk = false }

	status := "ok"
	code := 200
	if !dbOk {
		status = "degraded"
		code = 503
	}

	middleware.WriteJSON(w, code, map[string]interface{}{
		"status":   status,
		"database": dbOk,
	})
}

// Settings Handler
type SettingsHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewSettingsHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *SettingsHandler {
	return &SettingsHandler{DB: db, Auth: authSvc}
}

func (h *SettingsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/", h.ListSettings)
	r.Put("/", h.UpdateSettings)
	return r
}

func (h *SettingsHandler) ListSettings(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT key, value, description, category, updated_at FROM system_settings ORDER BY category, key`)
	defer rows.Close()
	var settings []map[string]interface{}
	for rows.Next() {
		var key, value, category string; var desc *string; var ts time.Time
		rows.Scan(&key, &value, &desc, &category, &ts)
		var val interface{}
		json.Unmarshal([]byte(value), &val)
		settings = append(settings, map[string]interface{}{
			"key": key, "value": val, "description": desc,
			"category": category, "updated_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, settings)
}

func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	userID := middleware.GetUserID(r.Context())
	for key, val := range req {
		valJSON, _ := json.Marshal(val)
		h.DB.Exec(r.Context(),
			`UPDATE system_settings SET value=$1, updated_by=$2, updated_at=NOW() WHERE key=$3`,
			valJSON, userID, key)
	}
	middleware.WriteJSON(w, 200, map[string]string{"message": "Settings updated"})
}

// Notification Handler
type NotificationHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewNotificationHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *NotificationHandler {
	return &NotificationHandler{DB: db, Auth: authSvc}
}

func (h *NotificationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/channels", h.ListChannels)
	r.Post("/channels", h.CreateChannel)
	r.Get("/deliveries", h.ListDeliveries)
	return r
}

func (h *NotificationHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, name, type, enabled, created_at FROM notification_channels ORDER BY name`)
	defer rows.Close()
	var channels []map[string]interface{}
	for rows.Next() {
		var id, name, ntype string; var enabled bool; var ts time.Time
		rows.Scan(&id, &name, &ntype, &enabled, &ts)
		channels = append(channels, map[string]interface{}{
			"id": id, "name": name, "type": ntype, "enabled": enabled, "created_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, channels)
}

func (h *NotificationHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string                 `json:"name"`
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	configJSON, _ := json.Marshal(req.Config)
	var id string
	h.DB.QueryRow(r.Context(),
		`INSERT INTO notification_channels (name, type, config) VALUES ($1,$2,$3) RETURNING id`,
		req.Name, req.Type, configJSON).Scan(&id)
	middleware.WriteJSON(w, 201, map[string]string{"id": id})
}

func (h *NotificationHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, status, title, message, created_at, sent_at FROM notification_deliveries ORDER BY created_at DESC LIMIT 50`)
	defer rows.Close()
	var deliveries []map[string]interface{}
	for rows.Next() {
		var id, stat string; var title, msg *string; var created time.Time; var sent *time.Time
		rows.Scan(&id, &stat, &title, &msg, &created, &sent)
		deliveries = append(deliveries, map[string]interface{}{
			"id": id, "status": stat, "title": title, "message": msg,
			"created_at": created, "sent_at": sent,
		})
	}
	middleware.WriteJSON(w, 200, deliveries)
}

// Log Handler
type LogHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewLogHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *LogHandler {
	return &LogHandler{DB: db, Auth: authSvc}
}

func (h *LogHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/", h.ListLogs)
	r.Get("/sources", h.ListSources)
	return r
}

func (h *LogHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host_id")
	level := r.URL.Query().Get("level")
	search := r.URL.Query().Get("q")
	limit := parseIntParam(r, "limit", 100)

	query := `SELECT id, source_id, host_id, level, message, timestamp FROM logs`
	var args []interface{}
	var conds []string
	if hostID != "" {
		conds = append(conds, "host_id=$"+itoa(len(args)+1))
		args = append(args, hostID)
	}
	if level != "" {
		conds = append(conds, "level=$"+itoa(len(args)+1))
		args = append(args, level)
	}
	if search != "" {
		conds = append(conds, "message ILIKE $"+itoa(len(args)+1))
		args = append(args, "%"+search+"%")
	}
	if len(conds) > 0 {
		query += " WHERE " + joinStrings(conds, " AND ")
	}
	query += " ORDER BY timestamp DESC LIMIT $" + itoa(len(args)+1)
	args = append(args, limit)

	rows, _ := h.DB.Query(r.Context(), query, args...)
	defer rows.Close()
	var logs []map[string]interface{}
	for rows.Next() {
		var id, lvl, msg string; var srcID, hostID2 *string; var ts time.Time
		rows.Scan(&id, &srcID, &hostID2, &lvl, &msg, &ts)
		logs = append(logs, map[string]interface{}{
			"id": id, "source_id": srcID, "host_id": hostID2,
			"level": lvl, "message": msg, "timestamp": ts,
		})
	}
	middleware.WriteJSON(w, 200, logs)
}

func (h *LogHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, host_id, name, type, enabled FROM log_sources ORDER BY name`)
	defer rows.Close()
	var sources []map[string]interface{}
	for rows.Next() {
		var id, name, ntype string; var hostID *string; var enabled bool
		rows.Scan(&id, &hostID, &name, &ntype, &enabled)
		sources = append(sources, map[string]interface{}{
			"id": id, "host_id": hostID, "name": name, "type": ntype, "enabled": enabled,
		})
	}
	middleware.WriteJSON(w, 200, sources)
}

// Helper
func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 { result += sep }
		result += s
	}
	return result
}
