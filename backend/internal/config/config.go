package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppName     string
	AppEnv      string
	AppDebug    bool
	AppPort     string
	FrontendURL string

	DatabaseURL string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string

	JWTSecret        string
	SessionSecret    string
	SessionMaxAge    time.Duration
	TOTPEncryptionKey string
	EncryptionKey    string

	AdminEmail    string
	AdminPassword string

	SMTPhost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	TelegramBotToken  string
	DiscordWebhookURL string
	SlackWebhookURL   string

	AgentHeartbeatInterval int
	AgentServerURL         string

	NmapPath         string
	ScanMaxConcurrent int
	ScanDefaultTimeout int

	RateLimitLogin int
	RateLimitAPI   int

	MetricRetentionDays  int
	LogRetentionDays     int
	AlertRetentionDays   int
	ScanRetentionDays    int
	AuditRetentionDays   int
}

func Load() *Config {
	return &Config{
		AppName:     getEnv("APP_NAME", "IDmonitor"),
		AppEnv:      getEnv("APP_ENV", "development"),
		AppDebug:    getEnv("APP_DEBUG", "false") == "true",
		AppPort:     getEnv("APP_PORT", "8080"),
		FrontendURL: getEnv("APP_FRONTEND_URL", "http://localhost:3000"),

		DatabaseURL: getEnv("DATABASE_URL", ""),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "idmonitor"),
		DBPassword:  getEnv("DB_PASSWORD", "changeme"),
		DBName:      getEnv("DB_NAME", "idmonitor"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),

		JWTSecret:        getEnv("JWT_SECRET", "change-me-jwt-secret-64-chars-required-here!!"),
		SessionSecret:    getEnv("SESSION_SECRET", "change-me-session-secret-64-chars-here!!"),
		SessionMaxAge:    getDuration("SESSION_MAX_AGE", 86400),
		TOTPEncryptionKey: getEnv("TOTP_ENCRYPTION_KEY", "change-me-totp-encryption-32-chars!"),
		EncryptionKey:    getEnv("ENCRYPTION_KEY", "change-me-encryption-key-32-chars!!"),

		AdminEmail:    getEnv("IDMONITOR_ADMIN_EMAIL", "admin@idmonitor.local"),
		AdminPassword: getEnv("IDMONITOR_ADMIN_PASSWORD", "changeme"),

		SMTPhost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@idmonitor.local"),

		TelegramBotToken:  getEnv("TELEGRAM_BOT_TOKEN", ""),
		DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", ""),
		SlackWebhookURL:   getEnv("SLACK_WEBHOOK_URL", ""),

		AgentHeartbeatInterval: getEnvInt("AGENT_HEARTBEAT_INTERVAL", 30),
		AgentServerURL:         getEnv("AGENT_SERVER_URL", "http://localhost:8080"),

		NmapPath:          getEnv("NMAP_PATH", "/usr/bin/nmap"),
		ScanMaxConcurrent: getEnvInt("SCAN_MAX_CONCURRENT", 3),
		ScanDefaultTimeout: getEnvInt("SCAN_DEFAULT_TIMEOUT", 300),

		RateLimitLogin: getEnvInt("RATE_LIMIT_LOGIN", 5),
		RateLimitAPI:   getEnvInt("RATE_LIMIT_API", 100),

		MetricRetentionDays: getEnvInt("METRIC_RETENTION_DAYS", 90),
		LogRetentionDays:    getEnvInt("LOG_RETENTION_DAYS", 30),
		AlertRetentionDays:  getEnvInt("ALERT_RETENTION_DAYS", 365),
		ScanRetentionDays:   getEnvInt("SCAN_RETENTION_DAYS", 90),
		AuditRetentionDays:  getEnvInt("AUDIT_RETENTION_DAYS", 365),
	}
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

func getDuration(key string, fallbackSeconds int) time.Duration {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return time.Duration(i) * time.Second
		}
	}
	return time.Duration(fallbackSeconds) * time.Second
}
