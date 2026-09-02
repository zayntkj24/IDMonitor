package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	ServerURL        string
	AgentToken       string
	AgentName        string
	HeartbeatSeconds int
}

type MetricData struct {
	Type  string             `json:"type"`
	Name  string             `json:"name"`
	Value float64            `json:"value"`
	Unit  string             `json:"unit"`
	Tags  map[string]string  `json:"tags"`
}

type ProcessInfo struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu_usage"`
	Memory  float64 `json:"memory_usage"`
	Status  string  `json:"status"`
	Command string  `json:"command"`
}

type ServiceInfo struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Status      string  `json:"status"`
	Type        string  `json:"type"`
	CPU         float64 `json:"cpu_usage"`
	Memory      float64 `json:"memory_usage"`
}

type SoftwareInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Publisher string `json:"publisher"`
}

var config Config
var hostID string

func main() {
	config = Config{
		ServerURL:        getEnv("AGENT_SERVER_URL", "http://localhost:8080"),
		AgentToken:       os.Getenv("AGENT_TOKEN"),
		AgentName:        getEnv("AGENT_NAME", getHostname()),
		HeartbeatSeconds: getEnvInt("AGENT_HEARTBEAT_INTERVAL", 30),
	}

	if config.AgentToken == "" {
		log.Println("No AGENT_TOKEN set. Registering new agent...")
		register()
	}

	log.Printf("IDmonitor Agent started. Server: %s", config.ServerURL)

	// Start collectors
	go heartbeatLoop()
	go metricsLoop()
	go servicesLoop()
	go processesLoop()
	go softwareLoop()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Agent shutting down")
}

func register() {
	payload := map[string]interface{}{
		"name":      config.AgentName,
		"hostname":  getHostname(),
		"version":   "1.0.0",
		"os":        runtime.GOOS,
		"os_version": getOSVersion(),
		"ip_address": getLocalIP(),
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(config.ServerURL+"/api/v1/agents/register", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	config.AgentToken = result["token"].(string)
	hostID = result["host_id"].(string)

	// Save token
	os.Setenv("AGENT_TOKEN", config.AgentToken)
	log.Printf("Agent registered. Token: %s...", config.AgentToken[:16])
}

func heartbeatLoop() {
	for {
		payload := map[string]interface{}{
			"agent_id":    getAgentID(),
			"hostname":    getHostname(),
			"version":     "1.0.0",
			"cpu_usage":   getCPUUsage(),
			"memory_usage": getMemoryUsage(),
			"disk_usage":  getDiskUsage(),
		}
		sendJSON("/api/v1/agents/heartbeat", payload)
		time.Sleep(time.Duration(config.HeartbeatSeconds) * time.Second)
	}
}

func metricsLoop() {
	for {
		metrics := collectMetrics()
		payload := map[string]interface{}{
			"agent_id": getAgentID(),
			"host_id":  getHostID(),
			"metrics":  metrics,
		}
		sendJSON("/api/v1/agents/metrics", payload)
		time.Sleep(30 * time.Second)
	}
}

func servicesLoop() {
	for {
		services := collectServices()
		payload := map[string]interface{}{
			"agent_id":  getAgentID(),
			"host_id":   getHostID(),
			"services":  services,
		}
		sendJSON("/api/v1/agents/services", payload)
		time.Sleep(60 * time.Second)
	}
}

func processesLoop() {
	for {
		processes := collectProcesses()
		payload := map[string]interface{}{
			"agent_id":  getAgentID(),
			"host_id":   getHostID(),
			"processes": processes,
		}
		sendJSON("/api/v1/agents/processes", payload)
		time.Sleep(60 * time.Second)
	}
}

func softwareLoop() {
	for {
		packages := collectSoftware()
		payload := map[string]interface{}{
			"agent_id": getAgentID(),
			"host_id":  getHostID(),
			"packages": packages,
		}
		sendJSON("/api/v1/agents/software", payload)
		time.Sleep(300 * time.Second)
	}
}

func collectMetrics() []MetricData {
	var metrics []MetricData

	// CPU
	cpuUsage := getCPUUsage()
	metrics = append(metrics, MetricData{Type: "CPU", Name: "cpu_usage", Value: cpuUsage, Unit: "percent"})

	// Memory
	memUsage, memTotal, memUsed := getMemoryInfo()
	metrics = append(metrics, MetricData{Type: "MEMORY", Name: "memory_usage", Value: memUsage, Unit: "percent"})
	metrics = append(metrics, MetricData{Type: "MEMORY", Name: "memory_total", Value: float64(memTotal), Unit: "bytes"})
	metrics = append(metrics, MetricData{Type: "MEMORY", Name: "memory_used", Value: float64(memUsed), Unit: "bytes"})

	// Disk
	diskUsage, diskTotal, diskUsed := getDiskInfo("/")
	metrics = append(metrics, MetricData{Type: "DISK", Name: "disk_usage", Value: diskUsage, Unit: "percent"})
	metrics = append(metrics, MetricData{Type: "DISK", Name: "disk_total", Value: float64(diskTotal), Unit: "bytes"})
	metrics = append(metrics, MetricData{Type: "DISK", Name: "disk_used", Value: float64(diskUsed), Unit: "bytes"})

	// Load
	load1, load5, load15 := getLoadAverage()
	metrics = append(metrics, MetricData{Type: "LOAD", Name: "load_1m", Value: load1})
	metrics = append(metrics, MetricData{Type: "LOAD", Name: "load_5m", Value: load5})
	metrics = append(metrics, MetricData{Type: "LOAD", Name: "load_15m", Value: load15})

	// Network
	rx, tx := getNetworkIO()
	metrics = append(metrics, MetricData{Type: "NETWORK", Name: "rx_bytes", Value: float64(rx), Unit: "bytes"})
	metrics = append(metrics, MetricData{Type: "NETWORK", Name: "tx_bytes", Value: float64(tx), Unit: "bytes"})

	return metrics
}

func getCPUUsage() float64 {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("grep", "cpu ", "/proc/stat").Output()
		if err == nil {
			fields := strings.Fields(string(out))
			if len(fields) >= 5 {
				user, _ := strconv.ParseFloat(fields[1], 64)
				nice, _ := strconv.ParseFloat(fields[2], 64)
				system, _ := strconv.ParseFloat(fields[3], 64)
				idle, _ := strconv.ParseFloat(fields[4], 64)
				total := user + nice + system + idle
				if total > 0 {
					return ((total - idle) / total) * 100
				}
			}
		}
	}
	return 0
}

func getMemoryInfo() (percent float64, total int64, used int64) {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("free", "-b").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Mem:") {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						total, _ = strconv.ParseInt(fields[1], 10, 64)
						used, _ = strconv.ParseInt(fields[2], 10, 64)
						if total > 0 {
							percent = float64(used) / float64(total) * 100
						}
					}
				}
			}
		}
	}
	return
}

func getDiskInfo(path string) (percent float64, total int64, used int64) {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("df", "-B1", path).Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					total, _ = strconv.ParseInt(fields[1], 10, 64)
					used, _ = strconv.ParseInt(fields[2], 10, 64)
					if total > 0 {
						percent = float64(used) / float64(total) * 100
					}
				}
			}
		}
	}
	return
}

func getLoadAverage() (float64, float64, float64) {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("cat", "/proc/loadavg").Output()
		if err == nil {
			fields := strings.Fields(string(out))
			if len(fields) >= 3 {
				l1, _ := strconv.ParseFloat(fields[0], 64)
				l5, _ := strconv.ParseFloat(fields[1], 64)
				l15, _ := strconv.ParseFloat(fields[2], 64)
				return l1, l5, l15
			}
		}
	}
	return 0, 0, 0
}

func getNetworkIO() (rx int64, tx int64) {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("cat", "/proc/net/dev").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, ":") && !strings.HasPrefix(strings.TrimSpace(line), "Inter") && !strings.HasPrefix(strings.TrimSpace(line), "face") {
					parts := strings.Split(line, ":")
					if len(parts) == 2 {
						fields := strings.Fields(parts[1])
						if len(fields) >= 10 {
							r, _ := strconv.ParseInt(fields[0], 10, 64)
							t, _ := strconv.ParseInt(fields[8], 10, 64)
							rx += r
							tx += t
						}
					}
				}
			}
		}
	}
	return
}

func collectServices() []ServiceInfo {
	var services []ServiceInfo
	if runtime.GOOS == "linux" {
		out, err := exec.Command("systemctl", "list-units", "--type=service", "--no-pager", "--plain", "--no-legend").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					status := "UNKNOWN"
					if strings.Contains(fields[2], "active") {
						status = "RUNNING"
					} else if strings.Contains(fields[2], "inactive") || strings.Contains(fields[2], "failed") {
						status = "STOPPED"
					}
					services = append(services, ServiceInfo{
						Name:        fields[0],
						DisplayName: strings.Join(fields[3:], " "),
						Status:      status,
						Type:        "systemd",
					})
				}
			}
		}
	}
	return services
}

func collectProcesses() []ProcessInfo {
	var processes []ProcessInfo
	if runtime.GOOS == "linux" {
		out, err := exec.Command("ps", "aux", "--no-headers").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for i, line := range lines {
				if i > 50 { break } // Limit to top 50
				fields := strings.Fields(line)
				if len(fields) >= 11 {
					pid, _ := strconv.Atoi(fields[1])
					cpu, _ := strconv.ParseFloat(fields[2], 64)
					mem, _ := strconv.ParseFloat(fields[3], 64)
					processes = append(processes, ProcessInfo{
						PID:     pid,
						Name:    fields[10],
						User:    fields[0],
						CPU:     cpu,
						Memory:  mem,
						Status:  "running",
						Command: strings.Join(fields[10:], " "),
					})
				}
			}
		}
	}
	return processes
}

func collectSoftware() []SoftwareInfo {
	var packages []SoftwareInfo
	if runtime.GOOS == "linux" {
		out, err := exec.Command("dpkg", "-l").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "ii") {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						packages = append(packages, SoftwareInfo{
							Name:    fields[1],
							Version: fields[2],
						})
					}
				}
			}
		}
	}
	return packages
}

func sendJSON(path string, data interface{}) {
	body, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", config.ServerURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", config.AgentToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send to %s: %v", path, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Error from %s: %s", path, string(bodyBytes))
	}
}

func getAgentID() string {
	return os.Getenv("AGENT_ID")
}

func getHostID() string {
	if hostID != "" {
		return hostID
	}
	return os.Getenv("HOST_ID")
}

func getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func getOSVersion() string {
	if runtime.GOOS == "linux" {
		out, err := exec.Command("cat", "/etc/os-release").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				}
			}
		}
	}
	return runtime.GOOS
}

func getLocalIP() string {
	out, err := exec.Command("hostname", "-I").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func _ = fmt.Sprintf
