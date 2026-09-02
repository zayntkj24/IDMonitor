package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/idmonitor/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HostHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewHostHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *HostHandler {
	return &HostHandler{DB: db, Auth: authSvc}
}

func (h *HostHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/", h.ListHosts)
	r.Get("/{id}", h.GetHost)
	r.Get("/{id}/metrics", h.GetHostMetrics)
	r.Get("/{id}/interfaces", h.GetHostInterfaces)
	r.Get("/{id}/latest-metrics", h.GetLatestMetrics)
	return r
}

func (h *HostHandler) ListHosts(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 { page = 1 }
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 { limit = 25 }
	offset := (page - 1) * limit
	status := r.URL.Query().Get("status")

	countQ := `SELECT COUNT(*) FROM hosts`
	queryQ := `SELECT h.id, h.name, h.hostname, h.ip_address, h.os, h.status,
	           h.cpu_cores, h.total_memory, h.last_heartbeat, h.created_at,
	           COALESCE(a.name, '') as agent_name, COALESCE(a.status, 'OFFLINE') as agent_status
	           FROM hosts h LEFT JOIN agents a ON a.id = h.agent_id`

	args := []interface{}{}
	where := ` WHERE 1=1`
	if status != "" {
		where += ` AND h.status = $1`
		args = append(args, status)
	}
	countQ += where
	queryQ += where

	var total int
	h.DB.QueryRow(r.Context(), countQ, args...).Scan(&total)

	queryQ += ` ORDER BY h.created_at DESC LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := h.DB.Query(r.Context(), queryQ, args...)
	if err != nil {
		middleware.WriteError(w, 500, "Failed")
		return
	}
	defer rows.Close()

	var hosts []map[string]interface{}
	for rows.Next() {
		var id, name, status string
		var hostname, ip, osName *string
		var cpuCores *int
		var totalMem *int64
		var agentName, agentStatus string
		var lastHB time.Time
		var createdAt time.Time
		rows.Scan(&id, &name, &hostname, &ip, &osName, &status, &cpuCores, &totalMem, &lastHB, &createdAt, &agentName, &agentStatus)
		hosts = append(hosts, map[string]interface{}{
			"id": id, "name": name, "hostname": hostname, "ip_address": ip, "os": osName,
			"status": status, "cpu_cores": cpuCores, "total_memory": totalMem,
			"last_heartbeat": lastHB, "created_at": createdAt,
			"agent_name": agentName, "agent_status": agentStatus,
		})
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"hosts": hosts, "total": total, "page": page, "limit": limit,
	})
}

func (h *HostHandler) GetHost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var host struct {
		ID, Name, Status string
		Hostname, IP, OS, OSVer, Kernel, Arch *string
		CPU *int; TotalMem, TotalDisk *int64
		AgentID *string
		Metadata []byte
		CreatedAt time.Time
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, hostname, ip_address, os, os_version, kernel, architecture,
		        cpu_cores, total_memory, total_disk, status, metadata, agent_id, created_at
		 FROM hosts WHERE id = $1`, id).
		Scan(&host.ID, &host.Name, &host.Hostname, &host.IP, &host.OS, &host.OSVer,
			&host.Kernel, &host.Arch, &host.CPU, &host.TotalMem, &host.TotalDisk,
			&host.Status, &host.Metadata, &host.AgentID, &host.CreatedAt)
	if err != nil {
		middleware.WriteError(w, 404, "Host not found")
		return
	}

	// Get services
	svcRows, _ := h.DB.Query(r.Context(), `SELECT id, name, status, cpu_usage, memory_usage FROM services WHERE host_id = $1`, id)
	defer svcRows.Close()
	var services []map[string]interface{}
	for svcRows.Next() {
		var sid, sname, sstatus string
		var cpu, mem *float64
		svcRows.Scan(&sid, &sname, &sstatus, &cpu, &mem)
		services = append(services, map[string]interface{}{
			"id": sid, "name": sname, "status": sstatus, "cpu_usage": cpu, "memory_usage": mem,
		})
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id": host.ID, "name": host.Name, "hostname": host.Hostname, "ip_address": host.IP,
		"os": host.OS, "os_version": host.OSVer, "kernel": host.Kernel, "architecture": host.Arch,
		"cpu_cores": host.CPU, "total_memory": host.TotalMem, "total_disk": host.TotalDisk,
		"status": host.Status, "metadata": host.Metadata, "agent_id": host.AgentID,
		"created_at": host.CreatedAt, "services": services,
	})
}

func (h *HostHandler) GetHostMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	metricType := r.URL.Query().Get("type")
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours < 1 { hours = 24 }

	query := `SELECT metric_type, name, value, unit, recorded_at
	          FROM metrics WHERE host_id = $1 AND recorded_at > NOW() - $2::INTERVAL`
	args := []interface{}{id, strconv.Itoa(hours) + " hours"}

	if metricType != "" {
		query += ` AND metric_type = $3`
		args = append(args, metricType)
	}
	query += ` ORDER BY recorded_at DESC LIMIT 1000`

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		middleware.WriteError(w, 500, "Failed")
		return
	}
	defer rows.Close()

	var metrics []map[string]interface{}
	for rows.Next() {
		var mtype, mname string
		var value float64
		var unit *string
		var ts time.Time
		rows.Scan(&mtype, &mname, &value, &unit, &ts)
		metrics = append(metrics, map[string]interface{}{
			"type": mtype, "name": mname, "value": value, "unit": unit, "recorded_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, metrics)
}

func (h *HostHandler) GetHostInterfaces(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, name, mac_address, ip_address, netmask, gateway, is_up
		 FROM host_interfaces WHERE host_id = $1`, id)
	defer rows.Close()
	var ifaces []map[string]interface{}
	for rows.Next() {
		var iid, name string
		var mac, ip, nm, gw *string
		var up bool
		rows.Scan(&iid, &name, &mac, &ip, &nm, &gw, &up)
		ifaces = append(ifaces, map[string]interface{}{
			"id": iid, "name": name, "mac_address": mac, "ip_address": ip,
			"netmask": nm, "gateway": gw, "is_up": up,
		})
	}
	middleware.WriteJSON(w, 200, ifaces)
}

func (h *HostHandler) GetLatestMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, _ := h.DB.Query(r.Context(),
		`SELECT DISTINCT ON (metric_type, name) metric_type, name, value, unit, recorded_at
		 FROM metrics WHERE host_id = $1 ORDER BY metric_type, name, recorded_at DESC`, id)
	defer rows.Close()
	var metrics []map[string]interface{}
	for rows.Next() {
		var mtype, mname string
		var value float64
		var unit *string
		var ts time.Time
		rows.Scan(&mtype, &mname, &value, &unit, &ts)
		metrics = append(metrics, map[string]interface{}{
			"type": mtype, "name": mname, "value": value, "unit": unit, "recorded_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, metrics)
}

// Monitor Handler
type MonitorHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewMonitorHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *MonitorHandler {
	return &MonitorHandler{DB: db, Auth: authSvc}
}

func (h *MonitorHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))
	r.Get("/", h.ListMonitors)
	r.Post("/", h.CreateMonitor)
	r.Get("/{id}", h.GetMonitor)
	r.Put("/{id}", h.UpdateMonitor)
	r.Delete("/{id}", h.DeleteMonitor)
	r.Get("/{id}/checks", h.GetMonitorChecks)
	return r
}

func (h *MonitorHandler) ListMonitors(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, description, type, url, host, port, status, uptime_percentage,
		        interval_seconds, enabled, last_check, created_at
		 FROM monitors WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		middleware.WriteError(w, 500, "Failed")
		return
	}
	defer rows.Close()
	var monitors []map[string]interface{}
	for rows.Next() {
		var id, name, mtype, status string
		var desc, url, host *string
		var port *int
		var uptime float64
		var interval int
		var enabled bool
		var lastCheck *time.Time
		var createdAt time.Time
		rows.Scan(&id, &name, &desc, &mtype, &url, &host, &port, &status, &uptime,
			&interval, &enabled, &lastCheck, &createdAt)
		monitors = append(monitors, map[string]interface{}{
			"id": id, "name": name, "description": desc, "type": mtype, "url": url,
			"host": host, "port": port, "status": status, "uptime_percentage": uptime,
			"interval_seconds": interval, "enabled": enabled,
			"last_check": lastCheck, "created_at": createdAt,
		})
	}
	middleware.WriteJSON(w, 200, monitors)
}

func (h *MonitorHandler) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		URL         string `json:"url"`
		Host        string `json:"host"`
		Port        *int   `json:"port"`
		Method      string `json:"method"`
		Interval    int    `json:"interval_seconds"`
		Timeout     int    `json:"timeout_seconds"`
		Retries     int    `json:"retries"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Interval < 10 { req.Interval = 60 }
	if req.Timeout < 5 { req.Timeout = 30 }
	if req.Retries < 1 { req.Retries = 3 }

	var id string
	err := h.DB.QueryRow(r.Context(),
		`INSERT INTO monitors (name, description, type, url, host, port, method, interval_seconds, timeout_seconds, retries)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		req.Name, req.Description, req.Type, req.URL, req.Host, req.Port,
		req.Method, req.Interval, req.Timeout, req.Retries).Scan(&id)
	if err != nil {
		middleware.WriteError(w, 400, "Failed to create monitor")
		return
	}
	middleware.WriteJSON(w, 201, map[string]string{"id": id})
}

func (h *MonitorHandler) GetMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var m struct {
		ID, Name, Mtype, Status string
		Desc, URL, Host *string
		Port *int; Uptime float64
		Interval, Timeout, Retries int
		Enabled bool; LastCheck *time.Time
		CreatedAt time.Time
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, description, type, url, host, port, status, uptime_percentage,
		        interval_seconds, timeout_seconds, retries, enabled, last_check, created_at
		 FROM monitors WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&m.ID, &m.Name, &m.Desc, &m.URL, &m.URL, &m.Host, &m.Port,
			&m.Uptime, &m.Interval, &m.Timeout, &m.Retries, &m.Enabled, &m.LastCheck, &m.CreatedAt)
	if err != nil {
		middleware.WriteError(w, 404, "Not found")
		return
	}
	middleware.WriteJSON(w, 200, m)
}

func (h *MonitorHandler) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	h.DB.Exec(r.Context(), `UPDATE monitors SET updated_at = NOW() WHERE id = $1`, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Updated"})
}

func (h *MonitorHandler) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec(r.Context(), `UPDATE monitors SET deleted_at = NOW() WHERE id = $1`, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Deleted"})
}

func (h *MonitorHandler) GetMonitorChecks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, status, response_time_ms, status_code, error_message, checked_at
		 FROM monitor_checks WHERE monitor_id = $1 ORDER BY checked_at DESC LIMIT 100`, id)
	defer rows.Close()
	var checks []map[string]interface{}
	for rows.Next() {
		var cid, status string
		var respTime *int; var statusCode *int
		var errMsg *string; var ts time.Time
		rows.Scan(&cid, &status, &respTime, &statusCode, &errMsg, &ts)
		checks = append(checks, map[string]interface{}{
			"id": cid, "status": status, "response_time_ms": respTime,
			"status_code": statusCode, "error_message": errMsg, "checked_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, checks)
}
