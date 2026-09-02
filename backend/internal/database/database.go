package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect(databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		databaseURL = "postgres://idmonitor:changeme@localhost:5432/idmonitor?sslmode=disable"
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	Pool = pool
	log.Println("Database connected successfully")
	return pool, nil
}

func Close() {
	if Pool != nil {
		Pool.Close()
		log.Println("Database connection closed")
	}
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationSQL := `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$ BEGIN
    CREATE TYPE user_status AS ENUM ('ACTIVE', 'DISABLED', 'LOCKED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE agent_status_enum AS ENUM ('ONLINE', 'OFFLINE', 'DEGRADED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE monitor_type AS ENUM ('HTTP', 'HTTPS', 'TCP', 'UDP', 'ICMP', 'DNS', 'PORT');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE monitor_status AS ENUM ('UP', 'DOWN', 'PENDING', 'MAINTENANCE');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE alert_severity AS ENUM ('INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE alert_status AS ENUM ('OPEN', 'ACKNOWLEDGED', 'RESOLVED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE incident_status AS ENUM ('OPEN', 'INVESTIGATING', 'CONTAINED', 'RESOLVED', 'CLOSED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE scan_profile_type AS ENUM ('HOST_DISCOVERY', 'SERVICE_DISCOVERY', 'COMMON_PORTS', 'FULL_TCP_PORT_SCAN', 'CUSTOM');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE scan_status AS ENUM ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE security_event_severity AS ENUM ('INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE fim_event_type AS ENUM ('CREATED', 'MODIFIED', 'DELETED', 'PERMISSION_CHANGED', 'OWNER_CHANGED', 'HASH_CHANGED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE vuln_status AS ENUM ('OPEN', 'MITIGATED', 'FIXED', 'ACCEPTED', 'FALSE_POSITIVE');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE service_status AS ENUM ('RUNNING', 'STOPPED', 'FAILED', 'UNKNOWN');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE notification_channel_type AS ENUM ('EMAIL', 'WEBHOOK', 'TELEGRAM', 'DISCORD', 'SLACK');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE notification_status AS ENUM ('QUEUED', 'SENT', 'FAILED', 'RETRY');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
DO $$ BEGIN
    CREATE TYPE metric_type AS ENUM ('CPU', 'MEMORY', 'SWAP', 'DISK', 'FILESYSTEM', 'DISK_IO', 'NETWORK', 'LOAD', 'PROCESS', 'SERVICE', 'UPTIME');
EXCEPTION WHEN duplicate_object THEN null;
END $$;
`
	_, err := pool.Exec(ctx, migrationSQL)
	if err != nil {
		return fmt.Errorf("failed to create types: %w", err)
	}

	// Create tables
	tablesSQL := `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(200),
    status user_status NOT NULL DEFAULT 'ACTIVE',
    two_factor_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    two_factor_secret VARCHAR(255),
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_login TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    UNIQUE(resource, action)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(512) NOT NULL UNIQUE,
    ip_address INET,
    user_agent TEXT,
    is_2fa_verified BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS login_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    email VARCHAR(255) NOT NULL,
    success BOOLEAN NOT NULL,
    ip_address INET,
    user_agent TEXT,
    failure_reason VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_recovery_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(255) NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    hostname VARCHAR(255),
    token VARCHAR(512) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'OFFLINE',
    version VARCHAR(50),
    os VARCHAR(100),
    os_version VARCHAR(100),
    ip_address INET,
    metadata JSONB DEFAULT '{}',
    last_heartbeat TIMESTAMPTZ,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS hosts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    name VARCHAR(200) NOT NULL,
    hostname VARCHAR(255),
    ip_address INET,
    os VARCHAR(100),
    os_version VARCHAR(100),
    kernel VARCHAR(200),
    architecture VARCHAR(50),
    cpu_cores INT,
    total_memory BIGINT,
    total_disk BIGINT,
    status VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS host_interfaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    mac_address VARCHAR(17),
    ip_address INET,
    netmask INET,
    gateway INET,
    is_up BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    display_name VARCHAR(200),
    status VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN',
    type VARCHAR(50),
    pid INT,
    cpu_usage REAL,
    memory_usage REAL,
    metadata JSONB DEFAULT '{}',
    last_checked TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS service_status_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS process_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    pid INT NOT NULL,
    name VARCHAR(200),
    user_name VARCHAR(100),
    cpu_usage REAL,
    memory_usage REAL,
    status VARCHAR(20),
    command TEXT,
    started_at TIMESTAMPTZ,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    metric_type metric_type NOT NULL,
    name VARCHAR(200) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(50),
    tags JSONB DEFAULT '{}',
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS monitors (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    type monitor_type NOT NULL,
    url VARCHAR(2048),
    host VARCHAR(255),
    port INT,
    method VARCHAR(10) DEFAULT 'GET',
    expected_status INT,
    expected_content TEXT,
    keywords TEXT[],
    interval_seconds INT NOT NULL DEFAULT 60,
    timeout_seconds INT NOT NULL DEFAULT 30,
    retries INT NOT NULL DEFAULT 3,
    status monitor_status NOT NULL DEFAULT 'PENDING',
    uptime_percentage REAL DEFAULT 100,
    last_check TIMESTAMPTZ,
    last_status monitor_status,
    consecutive_failures INT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS monitor_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status monitor_status NOT NULL,
    response_time_ms INT,
    status_code INT,
    response_body TEXT,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS log_sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    type VARCHAR(50) NOT NULL,
    path VARCHAR(1024),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES log_sources(id) ON DELETE CASCADE,
    host_id UUID REFERENCES hosts(id) ON DELETE CASCADE,
    level VARCHAR(20) NOT NULL DEFAULT 'INFO',
    message TEXT NOT NULL,
    fields JSONB DEFAULT '{}',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS security_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    condition_json JSONB NOT NULL,
    severity security_event_severity NOT NULL DEFAULT 'MEDIUM',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS security_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id UUID REFERENCES security_rules(id) ON DELETE SET NULL,
    host_id UUID REFERENCES hosts(id) ON DELETE SET NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity security_event_severity NOT NULL DEFAULT 'MEDIUM',
    source VARCHAR(200),
    source_event_id VARCHAR(200),
    details JSONB DEFAULT '{}',
    acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fim_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    path VARCHAR(1024) NOT NULL,
    recursive BOOLEAN NOT NULL DEFAULT FALSE,
    include_patterns TEXT[],
    exclude_patterns TEXT[],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fim_baselines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id UUID NOT NULL REFERENCES fim_rules(id) ON DELETE CASCADE,
    file_path VARCHAR(1024) NOT NULL,
    file_hash VARCHAR(255),
    size BIGINT,
    permissions VARCHAR(20),
    owner VARCHAR(100),
    last_modified TIMESTAMPTZ,
    baseline_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fim_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id UUID NOT NULL REFERENCES fim_rules(id) ON DELETE CASCADE,
    host_id UUID REFERENCES hosts(id) ON DELETE SET NULL,
    file_path VARCHAR(1024) NOT NULL,
    event_type fim_event_type NOT NULL,
    old_hash VARCHAR(255),
    new_hash VARCHAR(255),
    old_permissions VARCHAR(20),
    new_permissions VARCHAR(20),
    details JSONB DEFAULT '{}',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS software_inventory (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(300) NOT NULL,
    version VARCHAR(100),
    publisher VARCHAR(300),
    installed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vulnerabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cve_id VARCHAR(50),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity security_event_severity NOT NULL,
    cvss_score REAL,
    affected_software TEXT,
    reference_url VARCHAR(2048),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vulnerability_findings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    vulnerability_id UUID NOT NULL REFERENCES vulnerabilities(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    software_id UUID REFERENCES software_inventory(id) ON DELETE SET NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'OPEN',
    notes TEXT,
    found_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS scan_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    profile_type scan_profile_type NOT NULL,
    arguments JSONB DEFAULT '{}',
    description TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scan_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID REFERENCES scan_profiles(id) ON DELETE SET NULL,
    target VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'QUEUED',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration_ms INT,
    hosts_discovered INT DEFAULT 0,
    ports_discovered INT DEFAULT 0,
    raw_output TEXT,
    error_message TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS discovered_hosts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scan_job_id UUID NOT NULL REFERENCES scan_jobs(id) ON DELETE CASCADE,
    ip_address INET NOT NULL,
    hostname VARCHAR(255),
    mac_address VARCHAR(17),
    os_guess VARCHAR(200),
    state VARCHAR(50),
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS discovered_ports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES discovered_hosts(id) ON DELETE CASCADE,
    port INT NOT NULL,
    protocol VARCHAR(10) NOT NULL DEFAULT 'tcp',
    state VARCHAR(50) NOT NULL,
    service VARCHAR(200),
    product VARCHAR(200),
    version VARCHAR(100),
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scan_changes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    previous_scan_id UUID REFERENCES scan_jobs(id),
    current_scan_id UUID NOT NULL REFERENCES scan_jobs(id),
    change_type VARCHAR(50) NOT NULL,
    host_ip INET,
    port_number INT,
    protocol VARCHAR(10),
    details JSONB DEFAULT '{}',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity alert_severity NOT NULL DEFAULT 'MEDIUM',
    status alert_status NOT NULL DEFAULT 'OPEN',
    source VARCHAR(200),
    source_id UUID,
    host_id UUID REFERENCES hosts(id) ON DELETE SET NULL,
    monitor_id UUID REFERENCES monitors(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}',
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity alert_severity NOT NULL DEFAULT 'MEDIUM',
    status VARCHAR(30) NOT NULL DEFAULT 'OPEN',
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    host_id UUID REFERENCES hosts(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}',
    created_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incident_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id UUID REFERENCES alerts(id) ON DELETE SET NULL,
    security_event_id UUID REFERENCES security_events(id) ON DELETE SET NULL,
    event_type VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incident_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    type VARCHAR(20) NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_types TEXT[] NOT NULL,
    severity_filter TEXT[],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id UUID REFERENCES notification_rules(id) ON DELETE SET NULL,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'QUEUED',
    title VARCHAR(500),
    message TEXT,
    payload JSONB DEFAULT '{}',
    error_message TEXT,
    retry_count INT DEFAULT 0,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    recurrence VARCHAR(100),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS maintenance_targets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    maintenance_id UUID NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_email VARCHAR(255),
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(100),
    target_id UUID,
    details JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS system_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(200) NOT NULL UNIQUE,
    value JSONB NOT NULL,
    description TEXT,
    category VARCHAR(100),
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS dashboard_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    layout JSONB DEFAULT '{}',
    widgets JSONB DEFAULT '{}',
    theme VARCHAR(20) DEFAULT 'dark',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);
`
	_, err = pool.Exec(ctx, tablesSQL)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Create indexes
	idxSQL := `
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_login_history_user_id ON login_history(user_id);
CREATE INDEX IF NOT EXISTS idx_recovery_codes_user_id ON user_recovery_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_token ON agents(token);
CREATE INDEX IF NOT EXISTS idx_hosts_agent_id ON hosts(agent_id);
CREATE INDEX IF NOT EXISTS idx_hosts_status ON hosts(status);
CREATE INDEX IF NOT EXISTS idx_host_interfaces_host_id ON host_interfaces(host_id);
CREATE INDEX IF NOT EXISTS idx_services_host_id ON services(host_id);
CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);
CREATE INDEX IF NOT EXISTS idx_process_snapshots_host_id ON process_snapshots(host_id);
CREATE INDEX IF NOT EXISTS idx_metrics_host_type_time ON metrics(host_id, metric_type, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_monitors_status ON monitors(status);
CREATE INDEX IF NOT EXISTS idx_monitor_checks_monitor_time ON monitor_checks(monitor_id, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_logs_host_time ON logs(host_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_security_events_host_id ON security_events(host_id);
CREATE INDEX IF NOT EXISTS idx_security_events_severity ON security_events(severity);
CREATE INDEX IF NOT EXISTS idx_fim_events_rule_id ON fim_events(rule_id);
CREATE INDEX IF NOT EXISTS idx_fim_baselines_rule_id ON fim_baselines(rule_id);
CREATE INDEX IF NOT EXISTS idx_software_inventory_host_id ON software_inventory(host_id);
CREATE INDEX IF NOT EXISTS idx_vuln_findings_host_id ON vulnerability_findings(host_id);
CREATE INDEX IF NOT EXISTS idx_vuln_findings_status ON vulnerability_findings(status);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_status ON scan_jobs(status);
CREATE INDEX IF NOT EXISTS idx_discovered_hosts_ip ON discovered_hosts(ip_address);
CREATE INDEX IF NOT EXISTS idx_discovered_ports_host_id ON discovered_ports(host_id);
CREATE INDEX IF NOT EXISTS idx_scan_changes_current_scan ON scan_changes(current_scan_id);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_host_id ON alerts(host_id);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status ON notification_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target ON audit_logs(target_type, target_id);
`
	_, err = pool.Exec(ctx, idxSQL)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	// Seed default roles and permissions
	seedSQL := `
INSERT INTO permissions (resource, action, description) VALUES
('users','read','View users'),('users','create','Create users'),('users','update','Update users'),('users','delete','Delete users'),
('roles','read','View roles'),('roles','create','Create roles'),('roles','update','Update roles'),('roles','delete','Delete roles'),
('agents','read','View agents'),('agents','manage','Manage agents'),
('hosts','read','View hosts'),('hosts','create','Create hosts'),('hosts','update','Update hosts'),('hosts','delete','Delete hosts'),
('monitors','read','View monitors'),('monitors','create','Create monitors'),('monitors','update','Update monitors'),('monitors','delete','Delete monitors'),
('services','read','View services'),('services','manage','Manage services'),
('processes','read','View processes'),('metrics','read','View metrics'),
('logs','read','View logs'),('logs','manage','Manage log sources'),
('security','read','View security events'),('security','manage','Manage security'),
('fim','read','View FIM events'),('fim','manage','Manage FIM rules'),
('vulnerabilities','read','View vulnerabilities'),('vulnerabilities','manage','Manage vulnerabilities'),
('scanner','read','View scan results'),('scanner','run','Run scans'),('scanner','manage','Manage scan profiles'),
('alerts','read','View alerts'),('alerts','manage','Manage alerts'),
('incidents','read','View incidents'),('incidents','manage','Manage incidents'),
('notifications','read','View notifications'),('notifications','manage','Manage notifications'),
('audit','read','View audit logs'),
('settings','read','View settings'),('settings','manage','Manage settings'),
('reports','read','View reports'),('reports','export','Export reports')
ON CONFLICT (resource, action) DO NOTHING;

INSERT INTO roles (id, name, description, is_system) VALUES
('a0000000-0000-0000-0000-000000000001','ADMIN','Full system administrator',TRUE),
('a0000000-0000-0000-0000-000000000002','OPERATOR','Operations staff',TRUE),
('a0000000-0000-0000-0000-000000000003','VIEWER','Read-only access',TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0000000-0000-0000-0000-000000000001', id FROM permissions
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0000000-0000-0000-0000-000000000002', id FROM permissions WHERE action IN ('read','manage','run','create','update','export')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0000000-0000-0000-0000-000000000003', id FROM permissions WHERE action = 'read'
ON CONFLICT DO NOTHING;

INSERT INTO scan_profiles (name, profile_type, arguments, is_default) VALUES
('Host Discovery','HOST_DISCOVERY','{"args":["-sn"]}',TRUE),
('Service Discovery','SERVICE_DISCOVERY','{"args":["-sV","-O"]}',TRUE),
('Common Ports','COMMON_PORTS','{"args":["--top-ports","1000","-sV"]}',TRUE),
('Full TCP Port Scan','FULL_TCP_PORT_SCAN','{"args":["-p-","-sV"]}',TRUE)
ON CONFLICT DO NOTHING;

INSERT INTO system_settings (key, value, description, category) VALUES
('2fa_policy','"optional"','2FA enforcement policy','security'),
('session_timeout','86400','Session timeout seconds','security'),
('max_failed_logins','5','Max failed login attempts','security'),
('lockout_duration','900','Lockout duration seconds','security'),
('metric_retention_days','90','Metric data retention','retention'),
('log_retention_days','30','Log data retention','retention'),
('nmap_path','"/usr/bin/nmap"','Path to nmap','scanner'),
('max_concurrent_scans','3','Max concurrent scans','scanner')
ON CONFLICT (key) DO NOTHING;
`
	_, err = pool.Exec(ctx, seedSQL)
	if err != nil {
		return fmt.Errorf("failed to seed data: %w", err)
	}

	log.Println("Database migrations applied successfully")
	return nil
}
