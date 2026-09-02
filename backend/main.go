package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/idmonitor/backend/internal/auth"
	"github.com/idmonitor/backend/internal/config"
	"github.com/idmonitor/backend/internal/database"
	"github.com/idmonitor/backend/internal/handlers"
	"github.com/idmonitor/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting %s (env=%s)", cfg.AppName, cfg.AppEnv)

	// Connect to database
	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer database.Close()

	// Run migrations
	ctx := context.Background()
	if err := database.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Bootstrap admin user
	bootstrapAdmin(pool, cfg)

	// Initialize services
	authSvc := auth.NewAuthService(pool, cfg)
	totpSvc := auth.NewTOTPService(pool)

	// Initialize handlers
	authH := handlers.NewAuthHandler(pool, authSvc, totpSvc)
	userH := handlers.NewUserHandler(pool, authSvc)
	roleH := handlers.NewRoleHandler(pool, authSvc)
	agentH := handlers.NewAgentHandler(pool, authSvc)
	hostH := handlers.NewHostHandler(pool, authSvc)
	monitorH := handlers.NewMonitorHandler(pool, authSvc)
	scannerH := handlers.NewScannerHandler(pool, authSvc)
	alertH := handlers.NewAlertHandler(pool, authSvc)
	incidentH := handlers.NewIncidentHandler(pool, authSvc)
	securityH := handlers.NewSecurityHandler(pool, authSvc)
	auditH := handlers.NewAuditHandler(pool, authSvc)
	healthH := handlers.NewHealthHandler(pool)
	settingsH := handlers.NewSettingsHandler(pool, authSvc)
	notifH := handlers.NewNotificationHandler(pool, authSvc)
	logH := handlers.NewLogHandler(pool, authSvc)

	// Rate limiters
	loginLimiter := middleware.NewRateLimiter(cfg.RateLimitLogin, 1*time.Minute)
	apiLimiter := middleware.NewRateLimiter(cfg.RateLimitAPI, 1*time.Minute)

	// Build router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))

	// CORS
	origins := []string{"*"}
	if cfg.FrontendURL != "" {
		origins = strings.Split(cfg.FrontendURL, ",")
	}
	r.Use(middleware.CORS(origins))

	// Health endpoints (no auth)
	r.Mount("/api/health", healthH.Routes())

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public + rate limited)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RateLimit(loginLimiter))
			r.Mount("/auth", authH.Routes())
		})

		// Protected API routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Use(middleware.RateLimit(apiLimiter))

			r.Mount("/users", userH.Routes())
			r.Mount("/roles", roleH.Routes())
			r.Mount("/agents", agentH.Routes())
			r.Mount("/hosts", hostH.Routes())
			r.Mount("/monitors", monitorH.Routes())
			r.Mount("/scanner", scannerH.Routes())
			r.Mount("/alerts", alertH.Routes())
			r.Mount("/incidents", incidentH.Routes())
			r.Mount("/security", securityH.Routes())
			r.Mount("/audit", auditH.Routes())
			r.Mount("/settings", settingsH.Routes())
			r.Mount("/notifications", notifH.Routes())
			r.Mount("/logs", logH.Routes())
		})

		// Dashboard stats
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Get("/dashboard/stats", func(w http.ResponseWriter, r *http.Request) {
				stats := map[string]interface{}{}
				var hostsTotal, hostsOnline, agentsTotal, agentsOnline int
				var alertsOpen, incidentsOpen, vulnsOpen, scansTotal int

				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM hosts`).Scan(&hostsTotal)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM hosts WHERE status='ONLINE'`).Scan(&hostsOnline)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM agents WHERE deleted_at IS NULL`).Scan(&agentsTotal)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM agents WHERE status='ONLINE' AND deleted_at IS NULL`).Scan(&agentsOnline)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM alerts WHERE status IN ('OPEN','ACKNOWLEDGED')`).Scan(&alertsOpen)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM incidents WHERE status NOT IN ('RESOLVED','CLOSED')`).Scan(&incidentsOpen)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM vulnerability_findings WHERE status='OPEN'`).Scan(&vulnsOpen)
				pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM scan_jobs`).Scan(&scansTotal)

				stats["hosts"] = map[string]interface{}{"total": hostsTotal, "online": hostsOnline}
				stats["agents"] = map[string]interface{}{"total": agentsTotal, "online": agentsOnline}
				stats["alerts"] = map[string]interface{}{"open": alertsOpen}
				stats["incidents"] = map[string]interface{}{"open": incidentsOpen}
				stats["vulnerabilities"] = map[string]interface{}{"open": vulnsOpen}
				stats["scans"] = map[string]interface{}{"total": scansTotal}

				middleware.WriteJSON(w, 200, stats)
			})
		})
	})

	// Start background workers
	go startAgentStatusChecker(pool)
	go startMetricRetentionWorker(pool, cfg)

	// Server
	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("Server listening on :%s", cfg.AppPort)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func bootstrapAdmin(pool *pgxpool.Pool, cfg *config.Config) {
	var count int
	pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`).Scan(&count)
	if count > 0 {
		return
	}

	log.Println("No users found. Creating initial admin user...")

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Warning: failed to hash admin password: %v", err)
		return
	}

	var userID string
	err = pool.QueryRow(context.Background(),
		`INSERT INTO users (email, username, password_hash, display_name, status)
		 VALUES ($1, $2, $3, 'Administrator', 'ACTIVE')
		 ON CONFLICT (email) DO NOTHING
		 RETURNING id`,
		cfg.AdminEmail, "admin", string(hash)).Scan(&userID)
	if err != nil {
		// User already exists
		log.Printf("Admin user already exists or creation failed: %v", err)
		return
	}

	// Assign ADMIN role
	pool.Exec(context.Background(),
		`INSERT INTO user_roles (user_id, role_id)
		 SELECT $1, id FROM roles WHERE name = 'ADMIN'
		 ON CONFLICT DO NOTHING`, userID)

	log.Printf("Admin user created: %s", cfg.AdminEmail)
}

func startAgentStatusChecker(pool *pgxpool.Pool) {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		pool.Exec(context.Background(),
			`UPDATE agents SET status = 'OFFLINE' WHERE status = 'ONLINE' AND last_heartbeat < NOW() - INTERVAL '90 seconds' AND deleted_at IS NULL`)
		pool.Exec(context.Background(),
			`UPDATE hosts SET status = 'OFFLINE' WHERE agent_id IN
			 (SELECT id FROM agents WHERE status = 'OFFLINE') AND status = 'ONLINE'`)
	}
}

func startMetricRetentionWorker(pool *pgxpool.Pool, cfg *config.Config) {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		days := cfg.MetricRetentionDays
		pool.Exec(context.Background(),
			`DELETE FROM metrics WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL`, days)
		days2 := cfg.LogRetentionDays
		pool.Exec(context.Background(),
			`DELETE FROM logs WHERE timestamp < NOW() - ($1 || ' days')::INTERVAL`, days2)
	}
}
