package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/idmonitor/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewAgentHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *AgentHandler {
	return &AgentHandler{DB: db, Auth: authSvc}
}

func (h *AgentHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))

	r.Get("/", h.ListAgents)
	r.Get("/{id}", h.GetAgent)
	r.Delete("/{id}", h.DeleteAgent)

	// Agent API (authenticated by agent token)
	r.Post("/register", h.RegisterAgent)
	r.Post("/heartbeat", h.Heartbeat)
	r.Post("/metrics", h.IngestMetrics)
	r.Post("/services", h.IngestServices)
	r.Post("/processes", h.IngestProcesses)
	r.Post("/logs", h.IngestLogs)
	r.Post("/software", h.IngestSoftware)
	r.Post("/fim-events", h.IngestFIMEvents)
	return r
}

func (h *AgentHandler) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Agent-Token")
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, 401)
			return
		}
		var agentID string
		err := h.DB.QueryRow(r.Context(),
			`SELECT id FROM agents WHERE token = $1 AND deleted_at IS NULL`, token).Scan(&agentID)
		if err != nil {
			http.Error(w, `{"error":"invalid agent token"}`, 401)
			return
		}
		ctx := middleware.ContextWithUserID(r.Context(), agentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, hostname, status, version, os, os_version, ip_address,
		        last_heartbeat, registered_at, updated_at
		 FROM agents WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		middleware.WriteError(w, 500, "Failed")
		return
	}
	defer rows.Close()

	var agents []map[string]interface{}
	for rows.Next() {
		var id, name, status string
		var hostname, version, osName, osVer, ip *string
		var lastHB, registered, updated time.Time
		rows.Scan(&id, &name, &hostname, &status, &version, &osName, &osVer, &ip, &lastHB, &registered, &updated)
		agents = append(agents, map[string]interface{}{
			"id": id, "name": name, "hostname": hostname, "status": status,
			"version": version, "os": osName, "os_version": osVer, "ip_address": ip,
			"last_heartbeat": lastHB, "registered_at": registered, "updated_at": updated,
		})
	}
	middleware.WriteJSON(w, 200, agents)
}

func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var agent struct {
		ID, Name, Status string
		Hostname, Version, OSName, OSVer, IP *string
		LastHB, Registered, Updated time.Time
		Metadata []byte
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, hostname, status, version, os, os_version, ip_address,
		        last_heartbeat, registered_at, updated_at, metadata
		 FROM agents WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&agent.ID, &agent.Name, &agent.Hostname, &agent.Status, &agent.Version,
			&agent.OSName, &agent.OSVer, &agent.IP, &agent.LastHB, &agent.Registered, &agent.Updated, &agent.Metadata)
	if err != nil {
		middleware.WriteError(w, 404, "Agent not found")
		return
	}

	// Get associated host
	var hostID *string
	h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, id).Scan(&hostID)

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id": agent.ID, "name": agent.Name, "hostname": agent.Hostname,
		"status": agent.Status, "version": agent.Version, "os": agent.OSName,
		"os_version": agent.OSVer, "ip_address": agent.IP,
		"last_heartbeat": agent.LastHB, "registered_at": agent.Registered,
		"updated_at": agent.Updated, "metadata": agent.Metadata, "host_id": hostID,
	})
}

func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec(r.Context(), `UPDATE agents SET deleted_at = NOW() WHERE id = $1`, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Agent deleted"})
}

func (h *AgentHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string            `json:"name"`
		Hostname string            `json:"hostname"`
		Version  string            `json:"version"`
		OS       string            `json:"os"`
		OSVer    string            `json:"os_version"`
		IP       string            `json:"ip_address"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, 400, "Invalid request")
		return
	}

	token := uuid.New().String()
	var agentID string
	err := h.DB.QueryRow(r.Context(),
		`INSERT INTO agents (name, hostname, token, status, version, os, os_version, ip_address, metadata)
		 VALUES ($1, $2, $3, 'ONLINE', $4, $5, $6, $7, $8) RETURNING id`,
		req.Name, req.Hostname, token, req.Version, req.OS, req.OSVer, req.IP, req.Metadata).Scan(&agentID)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to register agent")
		return
	}

	// Create associated host
	var hostID string
	h.DB.QueryRow(r.Context(),
		`INSERT INTO hosts (agent_id, name, hostname, os, os_version, status)
		 VALUES ($1, $2, $3, $4, $5, 'ONLINE') RETURNING id`,
		agentID, req.Name, req.Hostname, req.OS, req.OSVer).Scan(&hostID)

	middleware.WriteJSON(w, 201, map[string]interface{}{
		"agent_id": agentID,
		"host_id":  hostID,
		"token":    token,
	})
}

func (h *AgentHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID   string `json:"agent_id"`
		Hostname  string `json:"hostname"`
		Version   string `json:"version"`
		Uptime    int64  `json:"uptime"`
		CPUUsage  float64 `json:"cpu_usage"`
		MemUsage  float64 `json:"memory_usage"`
		DiskUsage float64 `json:"disk_usage"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.DB.Exec(r.Context(),
		`UPDATE agents SET status = 'ONLINE', last_heartbeat = NOW(), hostname = $1, version = $2, updated_at = NOW()
		 WHERE id = $3`, req.Hostname, req.Version, req.AgentID)

	// Update host status
	h.DB.Exec(r.Context(),
		`UPDATE hosts SET status = 'ONLINE', hostname = $1, updated_at = NOW() WHERE agent_id = $2`,
		req.Hostname, req.AgentID)

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AgentHandler) IngestMetrics(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		HostID  string `json:"host_id"`
		Metrics []struct {
			Type string  `json:"type"`
			Name string  `json:"name"`
			Value float64 `json:"value"`
			Unit string  `json:"unit"`
			Tags map[string]string `json:"tags"`
		} `json:"metrics"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.HostID == "" {
		// Find host by agent
		h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID).Scan(&req.HostID)
	}

	if req.HostID == "" {
		middleware.WriteError(w, 400, "No host found for agent")
		return
	}

	for _, m := range req.Metrics {
		tagsJSON, _ := json.Marshal(m.Tags)
		h.DB.Exec(r.Context(),
			`INSERT INTO metrics (host_id, metric_type, name, value, unit, tags, recorded_at)
			 VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			req.HostID, m.Type, m.Name, m.Value, m.Unit, tagsJSON)
	}

	// Update host metrics snapshot
	if len(req.Metrics) > 0 {
		for _, m := range req.Metrics {
			switch m.Name {
			case "cpu_usage":
				h.DB.Exec(r.Context(), `UPDATE hosts SET metadata = metadata || jsonb_build_object('cpu_usage', $1) WHERE id = $2`, m.Value, req.HostID)
			case "memory_usage":
				h.DB.Exec(r.Context(), `UPDATE hosts SET metadata = metadata || jsonb_build_object('memory_usage', $1) WHERE id = $2`, m.Value, req.HostID)
			}
		}
	}

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AgentHandler) IngestServices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID  string `json:"agent_id"`
		HostID   string `json:"host_id"`
		Services []struct {
			Name        string  `json:"name"`
			DisplayName string  `json:"display_name"`
			Status      string  `json:"status"`
			Type        string  `json:"type"`
			PID         *int    `json:"pid"`
			CPU         float64 `json:"cpu_usage"`
			Memory      float64 `json:"memory_usage"`
		} `json:"services"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.HostID == "" {
		h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID).Scan(&req.HostID)
	}

	for _, svc := range req.Services {
		var svcID string
		h.DB.QueryRow(r.Context(),
			`INSERT INTO services (host_id, name, display_name, status, type, pid, cpu_usage, memory_usage, last_checked)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			 ON CONFLICT (host_id, name) DO UPDATE SET status = $4, cpu_usage = $7, memory_usage = $8, last_checked = NOW()
			 RETURNING id`,
			req.HostID, svc.Name, svc.DisplayName, svc.Status, svc.Type, svc.PID, svc.CPU, svc.Memory).Scan(&svcID)

		// Record status history
		h.DB.Exec(r.Context(),
			`INSERT INTO service_status_history (service_id, status, checked_at) VALUES ($1, $2, NOW())`,
			svcID, svc.Status)
	}

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AgentHandler) IngestProcesses(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID  string `json:"agent_id"`
		HostID   string `json:"host_id"`
		Processes []struct {
			PID     int     `json:"pid"`
			Name    string  `json:"name"`
			User    string  `json:"user"`
			CPU     float64 `json:"cpu_usage"`
			Memory  float64 `json:"memory_usage"`
			Status  string  `json:"status"`
			Command string  `json:"command"`
		} `json:"processes"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.HostID == "" {
		h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID).Scan(&req.HostID)
	}

	// Clear old snapshots for this host (keep latest batch only)
	h.DB.Exec(r.Context(), `DELETE FROM process_snapshots WHERE host_id = $1 AND captured_at < NOW() - INTERVAL '5 minutes'`, req.HostID)

	for _, proc := range req.Processes {
		h.DB.Exec(r.Context(),
			`INSERT INTO process_snapshots (host_id, pid, name, user_name, cpu_usage, memory_usage, status, command, captured_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
			req.HostID, proc.PID, proc.Name, proc.User, proc.CPU, proc.Memory, proc.Status, proc.Command)
	}

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AgentHandler) IngestLogs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		HostID  string `json:"host_id"`
		Logs    []struct {
			Source    string                 `json:"source"`
			Level     string                 `json:"level"`
			Message   string                 `json:"message"`
			Timestamp time.Time              `json:"timestamp"`
			Fields    map[string]interface{} `json:"fields"`
		} `json:"logs"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.HostID == "" {
		h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID).Scan(&req.HostID)
	}

	for _, log := range req.Logs {
		// Ensure source exists
		var sourceID string
		err := h.DB.QueryRow(r.Context(),
			`INSERT INTO log_sources (host_id, name, type) VALUES ($1, $2, 'agent')
			 ON CONFLICT DO NOTHING RETURNING id`, req.HostID, log.Source).Scan(&sourceID)
		if err != nil {
			h.DB.QueryRow(r.Context(),
				`SELECT id FROM log_sources WHERE host_id = $1 AND name = $2`, req.HostID, log.Source).Scan(&sourceID)
		}

		fieldsJSON, _ := json.Marshal(log.Fields)
		h.DB.Exec(r.Context(),
			`INSERT INTO logs (source_id, host_id, level, message, fields, timestamp)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			sourceID, req.HostID, log.Level, log.Message, fieldsJSON, log.Timestamp)
	}

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AgentHandler) IngestSoftware(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		HostID  string `json:"host_id"`
		Packages []struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			Publisher string `json:"publisher"`
		} `json:"packages"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.HostID == "" {
		h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID).Scan(&req.HostID)
	}

	// Clear old inventory
	h.DB.Exec(r.Context(), `DELETE FROM software_inventory WHERE host_id = $1`, req.HostID)

	for _, pkg := range req.Packages {
		h.DB.Exec(r.Context(),
			`INSERT INTO software_inventory (host_id, name, version, publisher) VALUES ($1, $2, $3, $4)`,
			req.HostID, pkg.Name, pkg.Version, pkg.Publisher)
	}

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AgentHandler) IngestFIMEvents(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentID string `json:"agent_id"`
		HostID  string `json:"host_id"`
		Events  []struct {
			FilePath string `json:"file_path"`
			Type     string `json:"type"`
			OldHash  string `json:"old_hash"`
			NewHash  string `json:"new_hash"`
		} `json:"events"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.HostID == "" {
		h.DB.QueryRow(r.Context(), `SELECT id FROM hosts WHERE agent_id = $1 LIMIT 1`, req.AgentID).Scan(&req.HostID)
	}

	for _, evt := range req.Events {
		// Find matching FIM rule
		var ruleID string
		h.DB.QueryRow(r.Context(),
			`SELECT id FROM fim_rules WHERE host_id = $1 AND enabled = TRUE AND $2 LIKE path || '%' LIMIT 1`,
			req.HostID, evt.FilePath).Scan(&ruleID)

		if ruleID == "" { continue }

		h.DB.Exec(r.Context(),
			`INSERT INTO fim_events (rule_id, host_id, file_path, event_type, old_hash, new_hash)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			ruleID, req.HostID, evt.FilePath, evt.Type, evt.OldHash, evt.NewHash)
	}

	middleware.WriteJSON(w, 200, map[string]string{"status": "ok"})
}
