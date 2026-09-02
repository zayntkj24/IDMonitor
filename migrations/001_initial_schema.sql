-- IDmonitor Database Schema
-- Migration 001: Initial Schema

BEGIN;

-- ============================================
-- EXTENSIONS
-- ============================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================
-- ENUM TYPES
-- ============================================
CREATE TYPE user_status AS ENUM ('ACTIVE', 'DISABLED', 'LOCKED');
CREATE TYPE agent_status AS ENUM ('ONLINE', 'OFFLINE', 'DEGRADED');
CREATE TYPE monitor_type AS ENUM ('HTTP', 'HTTPS', 'TCP', 'UDP', 'ICMP', 'DNS', 'PORT');
CREATE TYPE monitor_status AS ENUM ('UP', 'DOWN', 'PENDING', 'MAINTENANCE');
CREATE TYPE alert_severity AS ENUM ('INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL');
CREATE TYPE alert_status AS ENUM ('OPEN', 'ACKNOWLEDGED', 'RESOLVED');
CREATE TYPE incident_status AS ENUM ('OPEN', 'INVESTIGATING', 'CONTAINED', 'RESOLVED', 'CLOSED');
CREATE TYPE scan_profile AS ENUM ('HOST_DISCOVERY', 'SERVICE_DISCOVERY', 'COMMON_PORTS', 'FULL_TCP_PORT_SCAN', 'CUSTOM');
CREATE TYPE scan_status AS ENUM ('QUEUED', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED');
CREATE TYPE security_event_severity AS ENUM ('INFO', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL');
CREATE TYPE fim_event_type AS ENUM ('CREATED', 'MODIFIED', 'DELETED', 'PERMISSION_CHANGED', 'OWNER_CHANGED', 'HASH_CHANGED');
CREATE TYPE vuln_status AS ENUM ('OPEN', 'MITIGATED', 'FIXED', 'ACCEPTED', 'FALSE_POSITIVE');
CREATE TYPE service_status AS ENUM ('RUNNING', 'STOPPED', 'FAILED', 'UNKNOWN');
CREATE TYPE notification_channel_type AS ENUM ('EMAIL', 'WEBHOOK', 'TELEGRAM', 'DISCORD', 'SLACK');
CREATE TYPE notification_status AS ENUM ('QUEUED', 'SENT', 'FAILED', 'RETRY');
CREATE TYPE metric_type AS ENUM ('CPU', 'MEMORY', 'SWAP', 'DISK', 'FILESYSTEM', 'DISK_IO', 'NETWORK', 'LOAD', 'PROCESS', 'SERVICE', 'UPTIME');

-- ============================================
-- USERS & AUTH
-- ============================================
CREATE TABLE users (
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

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_status ON users(status);

-- ============================================
-- ROLES & PERMISSIONS
-- ============================================
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    UNIQUE(resource, action)
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- ============================================
-- SESSIONS
-- ============================================
CREATE TABLE sessions (
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

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ============================================
-- LOGIN HISTORY
-- ============================================
CREATE TABLE login_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    email VARCHAR(255) NOT NULL,
    success BOOLEAN NOT NULL,
    ip_address INET,
    user_agent TEXT,
    failure_reason VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_history_user_id ON login_history(user_id);
CREATE INDEX idx_login_history_created_at ON login_history(created_at);

-- ============================================
-- RECOVERY CODES
-- ============================================
CREATE TABLE user_recovery_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(255) NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recovery_codes_user_id ON user_recovery_codes(user_id);

-- ============================================
-- AGENTS
-- ============================================
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    hostname VARCHAR(255),
    token VARCHAR(512) NOT NULL UNIQUE,
    status agent_status NOT NULL DEFAULT 'OFFLINE',
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

CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_token ON agents(token);

-- ============================================
-- HOSTS
-- ============================================
CREATE TABLE hosts (
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

CREATE INDEX idx_hosts_agent_id ON hosts(agent_id);
CREATE INDEX idx_hosts_status ON hosts(status);

-- ============================================
-- HOST INTERFACES
-- ============================================
CREATE TABLE host_interfaces (
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

CREATE INDEX idx_host_interfaces_host_id ON host_interfaces(host_id);

-- ============================================
-- SERVICES
-- ============================================
CREATE TABLE services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    display_name VARCHAR(200),
    status service_status NOT NULL DEFAULT 'UNKNOWN',
    type VARCHAR(50),
    pid INT,
    cpu_usage REAL,
    memory_usage REAL,
    metadata JSONB DEFAULT '{}',
    last_checked TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_services_host_id ON services(host_id);
CREATE INDEX idx_services_status ON services(status);

-- ============================================
-- SERVICE STATUS HISTORY
-- ============================================
CREATE TABLE service_status_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    status service_status NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_service_status_history_service_id ON service_status_history(service_id);
CREATE INDEX idx_service_status_history_checked_at ON service_status_history(checked_at);

-- ============================================
-- PROCESS SNAPSHOTS
-- ============================================
CREATE TABLE process_snapshots (
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

CREATE INDEX idx_process_snapshots_host_id ON process_snapshots(host_id);
CREATE INDEX idx_process_snapshots_captured_at ON process_snapshots(captured_at);

-- ============================================
-- METRICS
-- ============================================
CREATE TABLE metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    metric_type metric_type NOT NULL,
    name VARCHAR(200) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(50),
    tags JSONB DEFAULT '{}',
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_metrics_host_id ON metrics(host_id);
CREATE INDEX idx_metrics_type ON metrics(metric_type);
CREATE INDEX idx_metrics_recorded_at ON metrics(recorded_at);
CREATE INDEX idx_metrics_host_type_time ON metrics(host_id, metric_type, recorded_at DESC);

-- ============================================
-- MONITORS (Uptime)
-- ============================================
CREATE TABLE monitors (
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

CREATE INDEX idx_monitors_status ON monitors(status);
CREATE INDEX idx_monitors_enabled ON monitors(enabled);

-- ============================================
-- MONITOR CHECKS
-- ============================================
CREATE TABLE monitor_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    status monitor_status NOT NULL,
    response_time_ms INT,
    status_code INT,
    response_body TEXT,
    error_message TEXT,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_monitor_checks_monitor_id ON monitor_checks(monitor_id);
CREATE INDEX idx_monitor_checks_checked_at ON monitor_checks(checked_at);
CREATE INDEX idx_monitor_checks_monitor_time ON monitor_checks(monitor_id, checked_at DESC);

-- ============================================
-- LOG SOURCES & LOGS
-- ============================================
CREATE TABLE log_sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    type VARCHAR(50) NOT NULL,
    path VARCHAR(1024),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES log_sources(id) ON DELETE CASCADE,
    host_id UUID REFERENCES hosts(id) ON DELETE CASCADE,
    level VARCHAR(20) NOT NULL DEFAULT 'INFO',
    message TEXT NOT NULL,
    fields JSONB DEFAULT '{}',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_logs_source_id ON logs(source_id);
CREATE INDEX idx_logs_host_id ON logs(host_id);
CREATE INDEX idx_logs_level ON logs(level);
CREATE INDEX idx_logs_timestamp ON logs(timestamp);
CREATE INDEX idx_logs_host_time ON logs(host_id, timestamp DESC);

-- ============================================
-- SECURITY RULES & EVENTS
-- ============================================
CREATE TABLE security_rules (
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

CREATE TABLE security_events (
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

CREATE INDEX idx_security_events_host_id ON security_events(host_id);
CREATE INDEX idx_security_events_severity ON security_events(severity);
CREATE INDEX idx_security_events_created_at ON security_events(created_at);
CREATE INDEX idx_security_events_acknowledged ON security_events(acknowledged);

-- ============================================
-- FILE INTEGRITY MONITORING
-- ============================================
CREATE TABLE fim_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    path VARCHAR(1024) NOT NULL,
    recursive BOOLEAN NOT NULL DEFAULT FALSE,
    include_patterns TEXT[],
    exclude_patterns TEXT[],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE fim_baselines (
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

CREATE INDEX idx_fim_baselines_rule_id ON fim_baselines(rule_id);

CREATE TABLE fim_events (
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

CREATE INDEX idx_fim_events_rule_id ON fim_events(rule_id);
CREATE INDEX idx_fim_events_detected_at ON fim_events(detected_at);

-- ============================================
-- SOFTWARE INVENTORY & VULNERABILITIES
-- ============================================
CREATE TABLE software_inventory (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name VARCHAR(300) NOT NULL,
    version VARCHAR(100),
    publisher VARCHAR(300),
    installed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_software_inventory_host_id ON software_inventory(host_id);

CREATE TABLE vulnerabilities (
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

CREATE TABLE vulnerability_findings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    vulnerability_id UUID NOT NULL REFERENCES vulnerabilities(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    software_id UUID REFERENCES software_inventory(id) ON DELETE SET NULL,
    status vuln_status NOT NULL DEFAULT 'OPEN',
    notes TEXT,
    found_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_vuln_findings_host_id ON vulnerability_findings(host_id);
CREATE INDEX idx_vuln_findings_status ON vulnerability_findings(status);

-- ============================================
-- NETWORK SCANNER
-- ============================================
CREATE TABLE scan_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    profile_type scan_profile NOT NULL,
    arguments JSONB DEFAULT '{}',
    description TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scan_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID REFERENCES scan_profiles(id) ON DELETE SET NULL,
    target VARCHAR(500) NOT NULL,
    status scan_status NOT NULL DEFAULT 'QUEUED',
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

CREATE INDEX idx_scan_jobs_status ON scan_jobs(status);
CREATE INDEX idx_scan_jobs_created_at ON scan_jobs(created_at);

CREATE TABLE discovered_hosts (
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

CREATE INDEX idx_discovered_hosts_ip ON discovered_hosts(ip_address);
CREATE INDEX idx_discovered_hosts_scan_job ON discovered_hosts(scan_job_id);

CREATE TABLE discovered_ports (
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

CREATE INDEX idx_discovered_ports_host_id ON discovered_ports(host_id);

CREATE TABLE scan_changes (
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

CREATE INDEX idx_scan_changes_current_scan ON scan_changes(current_scan_id);

-- ============================================
-- ALERTS
-- ============================================
CREATE TABLE alerts (
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

CREATE INDEX idx_alerts_status ON alerts(status);
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_host_id ON alerts(host_id);
CREATE INDEX idx_alerts_created_at ON alerts(created_at);

-- ============================================
-- INCIDENTS
-- ============================================
CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    severity alert_severity NOT NULL DEFAULT 'MEDIUM',
    status incident_status NOT NULL DEFAULT 'OPEN',
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    host_id UUID REFERENCES hosts(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}',
    created_by UUID REFERENCES users(id),
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);

CREATE TABLE incident_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id UUID REFERENCES alerts(id) ON DELETE SET NULL,
    security_event_id UUID REFERENCES security_events(id) ON DELETE SET NULL,
    event_type VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE incident_notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- NOTIFICATIONS
-- ============================================
CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    type notification_channel_type NOT NULL,
    config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_types TEXT[] NOT NULL,
    severity_filter alert_severity[],
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id UUID REFERENCES notification_rules(id) ON DELETE SET NULL,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    status notification_status NOT NULL DEFAULT 'QUEUED',
    title VARCHAR(500),
    message TEXT,
    payload JSONB DEFAULT '{}',
    error_message TEXT,
    retry_count INT DEFAULT 0,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_deliveries_status ON notification_deliveries(status);
CREATE INDEX idx_notification_deliveries_created_at ON notification_deliveries(created_at);

-- ============================================
-- MAINTENANCE WINDOWS
-- ============================================
CREATE TABLE maintenance_windows (
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

CREATE TABLE maintenance_targets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    maintenance_id UUID NOT NULL REFERENCES maintenance_windows(id) ON DELETE CASCADE,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL
);

-- ============================================
-- AUDIT LOG
-- ============================================
CREATE TABLE audit_logs (
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

CREATE INDEX idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_target ON audit_logs(target_type, target_id);

-- ============================================
-- SYSTEM SETTINGS
-- ============================================
CREATE TABLE system_settings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key VARCHAR(200) NOT NULL UNIQUE,
    value JSONB NOT NULL,
    description TEXT,
    category VARCHAR(100),
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================
-- DASHBOARD PREFERENCES
-- ============================================
CREATE TABLE dashboard_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    layout JSONB DEFAULT '{}',
    widgets JSONB DEFAULT '{}',
    theme VARCHAR(20) DEFAULT 'dark',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

-- ============================================
-- SEED DATA
-- ============================================

-- Default Permissions
INSERT INTO permissions (resource, action, description) VALUES
('users', 'read', 'View users'),
('users', 'create', 'Create users'),
('users', 'update', 'Update users'),
('users', 'delete', 'Delete users'),
('roles', 'read', 'View roles'),
('roles', 'create', 'Create roles'),
('roles', 'update', 'Update roles'),
('roles', 'delete', 'Delete roles'),
('agents', 'read', 'View agents'),
('agents', 'manage', 'Manage agents'),
('hosts', 'read', 'View hosts'),
('hosts', 'create', 'Create hosts'),
('hosts', 'update', 'Update hosts'),
('hosts', 'delete', 'Delete hosts'),
('monitors', 'read', 'View monitors'),
('monitors', 'create', 'Create monitors'),
('monitors', 'update', 'Update monitors'),
('monitors', 'delete', 'Delete monitors'),
('services', 'read', 'View services'),
('services', 'manage', 'Manage services'),
('processes', 'read', 'View processes'),
('metrics', 'read', 'View metrics'),
('logs', 'read', 'View logs'),
('logs', 'manage', 'Manage log sources'),
('security', 'read', 'View security events'),
('security', 'manage', 'Manage security rules and events'),
('fim', 'read', 'View FIM events'),
('fim', 'manage', 'Manage FIM rules'),
('vulnerabilities', 'read', 'View vulnerabilities'),
('vulnerabilities', 'manage', 'Manage vulnerability findings'),
('scanner', 'read', 'View scan results'),
('scanner', 'run', 'Run scans'),
('scanner', 'manage', 'Manage scan profiles'),
('alerts', 'read', 'View alerts'),
('alerts', 'manage', 'Manage alerts'),
('incidents', 'read', 'View incidents'),
('incidents', 'manage', 'Manage incidents'),
('notifications', 'read', 'View notifications'),
('notifications', 'manage', 'Manage notification channels'),
('audit', 'read', 'View audit logs'),
('settings', 'read', 'View settings'),
('settings', 'manage', 'Manage settings'),
('reports', 'read', 'View reports'),
('reports', 'export', 'Export reports');

-- Default Roles
INSERT INTO roles (id, name, description, is_system) VALUES
('a0000000-0000-0000-0000-000000000001', 'ADMIN', 'Full system administrator', TRUE),
('a0000000-0000-0000-0000-000000000002', 'OPERATOR', 'Operations staff with manage permissions', TRUE),
('a0000000-0000-0000-0000-000000000003', 'VIEWER', 'Read-only access', TRUE);

-- Admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0000000-0000-0000-0000-000000000001', id FROM permissions;

-- Operator gets read + manage
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0000000-0000-0000-0000-000000000002', id FROM permissions WHERE action IN ('read', 'manage', 'run', 'create', 'update', 'export');

-- Viewer gets read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT 'a0000000-0000-0000-0000-000000000003', id FROM permissions WHERE action = 'read';

-- Default Scan Profiles
INSERT INTO scan_profiles (name, profile_type, arguments, is_default) VALUES
('Host Discovery', 'HOST_DISCOVERY', '{"args": ["-sn"]}', TRUE),
('Service Discovery', 'SERVICE_DISCOVERY', '{"args": ["-sV", "-O"]}', TRUE),
('Common Ports', 'COMMON_PORTS', '{"args": ["--top-ports", "1000", "-sV"]}', TRUE),
('Full TCP Port Scan', 'FULL_TCP_PORT_SCAN', '{"args": ["-p-", "-sV"]}', TRUE);

-- Default System Settings
INSERT INTO system_settings (key, value, description, category) VALUES
('2fa_policy', '"optional"', '2FA enforcement policy', 'security'),
('session_timeout', '86400', 'Session timeout in seconds', 'security'),
('max_failed_logins', '5', 'Max failed login attempts before lockout', 'security'),
('lockout_duration', '900', 'Account lockout duration in seconds', 'security'),
('metric_retention_days', '90', 'Metric data retention period', 'retention'),
('log_retention_days', '30', 'Log data retention period', 'retention'),
('alert_retention_days', '365', 'Alert data retention period', 'retention'),
('scan_retention_days', '90', 'Scan history retention period', 'retention'),
('audit_retention_days', '365', 'Audit log retention period', 'retention'),
('agent_heartbeat_timeout', '90', 'Agent heartbeat timeout in seconds', 'agents'),
('nmap_path', '"/usr/bin/nmap"', 'Path to nmap binary', 'scanner'),
('max_concurrent_scans', '3', 'Maximum concurrent scan jobs', 'scanner');

COMMIT;
