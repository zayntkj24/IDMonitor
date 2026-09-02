package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/idmonitor/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScannerHandler struct {
	DB   *pgxpool.Pool
	Auth *auth.AuthService
}

func NewScannerHandler(db *pgxpool.Pool, authSvc *auth.AuthService) *ScannerHandler {
	return &ScannerHandler{DB: db, Auth: authSvc}
}

func (h *ScannerHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Auth(h.Auth))

	r.Get("/profiles", h.ListProfiles)
	r.Post("/profiles", h.CreateProfile)
	r.Post("/scan", h.StartScan)
	r.Get("/scans", h.ListScans)
	r.Get("/scans/{id}", h.GetScan)
	r.Delete("/scans/{id}", h.CancelScan)
	r.Get("/hosts", h.ListDiscoveredHosts)
	r.Get("/hosts/{id}/ports", h.GetHostPorts)
	r.Get("/changes", h.ListChanges)
	return r
}

var validTarget = regexp.MustCompile(`^([0-9a-fA-F\.\:\/]+|[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,})$`)

func validateTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" || len(target) > 255 {
		return false
	}
	return validTarget.MatchString(target)
}

func (h *ScannerHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, name, profile_type, arguments, description, is_default FROM scan_profiles ORDER BY name`)
	defer rows.Close()
	var profiles []map[string]interface{}
	for rows.Next() {
		var id, name, ptype string
		var args []byte
		var desc *string
		var def bool
		rows.Scan(&id, &name, &ptype, &args, &desc, &def)
		var argsMap map[string]interface{}
		json.Unmarshal(args, &argsMap)
		profiles = append(profiles, map[string]interface{}{
			"id": id, "name": name, "profile_type": ptype,
			"arguments": argsMap, "description": desc, "is_default": def,
		})
	}
	middleware.WriteJSON(w, 200, profiles)
}

func (h *ScannerHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string                 `json:"name"`
		Type        string                 `json:"profile_type"`
		Description string                 `json:"description"`
		Arguments   map[string]interface{} `json:"arguments"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	argsJSON, _ := json.Marshal(req.Arguments)
	var id string
	h.DB.QueryRow(r.Context(),
		`INSERT INTO scan_profiles (name, profile_type, arguments, description) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Name, req.Type, argsJSON, req.Description).Scan(&id)
	middleware.WriteJSON(w, 201, map[string]string{"id": id})
}

func (h *ScannerHandler) StartScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target    string `json:"target"`
		ProfileID string `json:"profile_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if !validateTarget(req.Target) {
		middleware.WriteError(w, 400, "Invalid target")
		return
	}

	userID := middleware.GetUserID(r.Context())

	var profileArgs []byte
	if req.ProfileID != "" {
		h.DB.QueryRow(r.Context(), `SELECT arguments FROM scan_profiles WHERE id = $1`, req.ProfileID).Scan(&profileArgs)
	} else {
		h.DB.QueryRow(r.Context(),
			`SELECT arguments FROM scan_profiles WHERE profile_type = 'COMMON_PORTS' AND is_default = TRUE LIMIT 1`).Scan(&profileArgs)
	}

	var jobID string
	err := h.DB.QueryRow(r.Context(),
		`INSERT INTO scan_jobs (profile_id, target, status, created_by) VALUES ($1, $2, 'QUEUED', $3) RETURNING id`,
		req.ProfileID, req.Target, userID).Scan(&jobID)
	if err != nil {
		middleware.WriteError(w, 500, "Failed to create scan job")
		return
	}

	go h.executeScan(jobID, req.Target, profileArgs)

	middleware.WriteJSON(w, 202, map[string]interface{}{
		"job_id": jobID, "status": "QUEUED", "target": req.Target,
	})
}

func (h *ScannerHandler) executeScan(jobID, target string, profileArgs []byte) {
	ctx := context.Background()
	h.DB.Exec(ctx, `UPDATE scan_jobs SET status = 'RUNNING', started_at = NOW() WHERE id = $1`, jobID)

	nmapPath := "/usr/bin/nmap"

	var argsMap map[string]interface{}
	json.Unmarshal(profileArgs, &argsMap)

	cmdArgs := []string{"-oX", "-", "--open"}
	if extraArgs, ok := argsMap["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			if s, ok := a.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}
	cmdArgs = append(cmdArgs, target)

	cmd := exec.CommandContext(ctx, nmapPath, cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		h.DB.Exec(ctx, `UPDATE scan_jobs SET status = 'FAILED', finished_at = NOW(), error_message = $1 WHERE id = $2`,
			err.Error(), jobID)
		return
	}

	outputStr := string(output)
	hosts, totalPorts := parseNmapOutput(outputStr)

	var hostsCount int
	for _, host := range hosts {
		var hostID string
		h.DB.QueryRow(ctx,
			`INSERT INTO discovered_hosts (scan_job_id, ip_address, hostname, os_guess, state)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			jobID, host.IP, host.Hostname, host.OS, host.State).Scan(&hostID)
		hostsCount++

		for _, port := range host.Ports {
			h.DB.Exec(ctx,
				`INSERT INTO discovered_ports (host_id, port, protocol, state, service, product, version)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				hostID, port.Port, port.Protocol, port.State, port.Service, port.Product, port.Version)
		}
	}

	h.DB.Exec(ctx,
		`UPDATE scan_jobs SET status = 'COMPLETED', finished_at = NOW(),
		        duration_ms = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000,
		        hosts_discovered = $1, ports_discovered = $2, raw_output = $3
		 WHERE id = $4`, hostsCount, totalPorts, outputStr, jobID)

	h.detectScanChanges(ctx, jobID)
}

func (h *ScannerHandler) detectScanChanges(ctx context.Context, jobID string) {
	var prevScanID string
	h.DB.QueryRow(ctx,
		`SELECT id FROM scan_jobs WHERE status = 'COMPLETED' AND id != $1
		 ORDER BY finished_at DESC LIMIT 1`, jobID).Scan(&prevScanID)
	if prevScanID == "" {
		return
	}

	prevHosts := make(map[string]bool)
	curHosts := make(map[string]bool)

	rows, _ := h.DB.Query(ctx, `SELECT ip_address FROM discovered_hosts WHERE scan_job_id = $1`, prevScanID)
	for rows.Next() {
		var ip string
		rows.Scan(&ip)
		prevHosts[ip] = true
	}
	rows.Close()

	rows, _ = h.DB.Query(ctx, `SELECT ip_address FROM discovered_hosts WHERE scan_job_id = $1`, jobID)
	for rows.Next() {
		var ip string
		rows.Scan(&ip)
		curHosts[ip] = true
	}
	rows.Close()

	for ip := range curHosts {
		if !prevHosts[ip] {
			h.DB.Exec(ctx,
				`INSERT INTO scan_changes (current_scan_id, change_type, host_ip, details)
				 VALUES ($1, 'NEW_HOST', $2, '{"message":"New host discovered"}')`, jobID, ip)
		}
	}
	for ip := range prevHosts {
		if !curHosts[ip] {
			h.DB.Exec(ctx,
				`INSERT INTO scan_changes (current_scan_id, change_type, host_ip, details)
				 VALUES ($1, 'REMOVED_HOST', $2, '{"message":"Host no longer found"}')`, jobID, ip)
		}
	}
}

func (h *ScannerHandler) ListScans(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, target, status, started_at, finished_at, duration_ms,
		        hosts_discovered, ports_discovered, created_at
		 FROM scan_jobs ORDER BY created_at DESC LIMIT 50`)
	defer rows.Close()
	var scans []map[string]interface{}
	for rows.Next() {
		var id, target, status string
		var started, finished *time.Time
		var duration *int
		var hCount, pCount int
		var created time.Time
		rows.Scan(&id, &target, &status, &started, &finished, &duration, &hCount, &pCount, &created)
		scans = append(scans, map[string]interface{}{
			"id": id, "target": target, "status": status,
			"started_at": started, "finished_at": finished, "duration_ms": duration,
			"hosts_discovered": hCount, "ports_discovered": pCount, "created_at": created,
		})
	}
	middleware.WriteJSON(w, 200, scans)
}

func (h *ScannerHandler) GetScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var scan struct {
		ID, Target, Status string
		Started, Finished  *time.Time
		Duration           *int
		HCount, PCount     int
		ErrorMsg           *string
		RawOutput          *string
		Created            time.Time
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, target, status, started_at, finished_at, duration_ms,
		        hosts_discovered, ports_discovered, error_message, raw_output, created_at
		 FROM scan_jobs WHERE id = $1`, id).
		Scan(&scan.ID, &scan.Target, &scan.Status, &scan.Started, &scan.Finished,
			&scan.Duration, &scan.HCount, &scan.PCount, &scan.ErrorMsg, &scan.RawOutput, &scan.Created)
	if err != nil {
		middleware.WriteError(w, 404, "Not found")
		return
	}

	hRows, _ := h.DB.Query(r.Context(), `SELECT id, ip_address, hostname, os_guess, state FROM discovered_hosts WHERE scan_job_id = $1`, id)
	defer hRows.Close()
	var hosts []map[string]interface{}
	for hRows.Next() {
		var hid, ip, state string
		var hostname, osGuess *string
		hRows.Scan(&hid, &ip, &hostname, &osGuess, &state)
		hosts = append(hosts, map[string]interface{}{
			"id": hid, "ip_address": ip, "hostname": hostname, "os_guess": osGuess, "state": state,
		})
	}

	cRows, _ := h.DB.Query(r.Context(), `SELECT id, change_type, host_ip, port_number, protocol, details, detected_at FROM scan_changes WHERE current_scan_id = $1`, id)
	defer cRows.Close()
	var changes []map[string]interface{}
	for cRows.Next() {
		var cid, cType, hip string
		var port *int
		var proto *string
		var details []byte
		var ts time.Time
		cRows.Scan(&cid, &cType, &hip, &port, &proto, &details, &ts)
		var detailsMap map[string]interface{}
		json.Unmarshal(details, &detailsMap)
		changes = append(changes, map[string]interface{}{
			"id": cid, "change_type": cType, "host_ip": hip,
			"port": port, "protocol": proto, "details": detailsMap, "detected_at": ts,
		})
	}

	middleware.WriteJSON(w, 200, map[string]interface{}{
		"id": scan.ID, "target": scan.Target, "status": scan.Status,
		"started_at": scan.Started, "finished_at": scan.Finished, "duration_ms": scan.Duration,
		"hosts_discovered": scan.HCount, "ports_discovered": scan.PCount,
		"error_message": scan.ErrorMsg, "created_at": scan.Created,
		"hosts": hosts, "changes": changes,
	})
}

func (h *ScannerHandler) CancelScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec(r.Context(), `UPDATE scan_jobs SET status = 'CANCELLED', finished_at = NOW() WHERE id = $1 AND status IN ('QUEUED','RUNNING')`, id)
	middleware.WriteJSON(w, 200, map[string]string{"message": "Cancelled"})
}

func (h *ScannerHandler) ListDiscoveredHosts(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, ip_address, hostname, mac_address, os_guess, state, discovered_at, last_seen
		 FROM discovered_hosts ORDER BY discovered_at DESC LIMIT 100`)
	defer rows.Close()
	var hosts []map[string]interface{}
	for rows.Next() {
		var id, ip, state string
		var hostname, mac, osGuess *string
		var discovered, lastSeen time.Time
		rows.Scan(&id, &ip, &hostname, &mac, &osGuess, &state, &discovered, &lastSeen)
		hosts = append(hosts, map[string]interface{}{
			"id": id, "ip_address": ip, "hostname": hostname, "mac_address": mac,
			"os_guess": osGuess, "state": state, "discovered_at": discovered, "last_seen": lastSeen,
		})
	}
	middleware.WriteJSON(w, 200, hosts)
}

func (h *ScannerHandler) GetHostPorts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, port, protocol, state, service, product, version
		 FROM discovered_ports WHERE host_id = $1 ORDER BY port`, id)
	defer rows.Close()
	var ports []map[string]interface{}
	for rows.Next() {
		var pid, proto, state string
		var port int
		var svc, prod, ver *string
		rows.Scan(&pid, &port, &proto, &state, &svc, &prod, &ver)
		ports = append(ports, map[string]interface{}{
			"id": pid, "port": port, "protocol": proto, "state": state,
			"service": svc, "product": prod, "version": ver,
		})
	}
	middleware.WriteJSON(w, 200, ports)
}

func (h *ScannerHandler) ListChanges(w http.ResponseWriter, r *http.Request) {
	rows, _ := h.DB.Query(r.Context(),
		`SELECT id, change_type, host_ip, port_number, protocol, details, detected_at
		 FROM scan_changes ORDER BY detected_at DESC LIMIT 100`)
	defer rows.Close()
	var changes []map[string]interface{}
	for rows.Next() {
		var id, cType, hip string
		var port *int
		var proto *string
		var details []byte
		var ts time.Time
		rows.Scan(&id, &cType, &hip, &port, &proto, &details, &ts)
		var d map[string]interface{}
		json.Unmarshal(details, &d)
		changes = append(changes, map[string]interface{}{
			"id": id, "change_type": cType, "host_ip": hip,
			"port": port, "protocol": proto, "details": d, "detected_at": ts,
		})
	}
	middleware.WriteJSON(w, 200, changes)
}

type nmapHost struct {
	IP       string
	Hostname string
	OS       string
	State    string
	Ports    []nmapPort
}

type nmapPort struct {
	Port     int
	Protocol string
	State    string
	Service  string
	Product  string
	Version  string
}

func parseNmapOutput(output string) ([]nmapHost, int) {
	var hosts []nmapHost
	totalPorts := 0
	lines := strings.Split(output, "\n")
	var currentHost *nmapHost

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Nmap scan report for") {
			if currentHost != nil {
				hosts = append(hosts, *currentHost)
			}
			parts := strings.Split(line, "for ")
			if len(parts) > 1 {
				addr := strings.TrimSpace(parts[1])
				addr = strings.Trim(addr, "()")
				currentHost = &nmapHost{IP: addr, State: "up"}
			}
		}
		if currentHost != nil && strings.Contains(line, "/tcp") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				portParts := strings.Split(fields[0], "/")
				port := 0
				for _, c := range portParts[0] {
					if c >= '0' && c <= '9' {
						port = port*10 + int(c-'0')
					}
				}
				p := nmapPort{
					Port:     port,
					Protocol: portParts[1],
					State:    fields[1],
				}
				if len(fields) > 2 {
					p.Service = fields[2]
				}
				if len(fields) > 3 {
					p.Product = fields[3]
				}
				if len(fields) > 4 {
					p.Version = strings.Join(fields[4:], " ")
				}
				currentHost.Ports = append(currentHost.Ports, p)
				totalPorts++
			}
		}
	}
	if currentHost != nil {
		hosts = append(hosts, *currentHost)
	}
	return hosts, totalPorts
}
