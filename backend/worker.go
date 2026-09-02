package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/idmonitor/backend/internal/config"
	"github.com/idmonitor/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()
	log.Println("IDmonitor Worker starting...")

	pool, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer database.Close()

	go agentStatusWorker(pool)
	go metricRetentionWorker(pool, cfg)
	go alertRetentionWorker(pool, cfg)
	go logRetentionWorker(pool, cfg)
	go notificationDeliveryWorker(pool)

	log.Println("Worker started successfully")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Worker shutting down...")
}

func agentStatusWorker(pool *pgxpool.Pool) {
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		ctx := context.Background()
		pool.Exec(ctx,
			`UPDATE agents SET status = 'OFFLINE' WHERE status = 'ONLINE' AND last_heartbeat < NOW() - INTERVAL '90 seconds' AND deleted_at IS NULL`)
		pool.Exec(ctx,
			`UPDATE hosts SET status = 'OFFLINE' WHERE agent_id IN (SELECT id FROM agents WHERE status = 'OFFLINE') AND status = 'ONLINE'`)
		log.Println("Agent status check completed")
	}
}

func metricRetentionWorker(pool *pgxpool.Pool, cfg *config.Config) {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		ctx := context.Background()
		days := cfg.MetricRetentionDays
		pool.Exec(ctx,
			`DELETE FROM metrics WHERE recorded_at < NOW() - ($1 || ' days')::INTERVAL`, days)
		log.Println("Metric retention cleanup completed")
	}
}

func alertRetentionWorker(pool *pgxpool.Pool, cfg *config.Config) {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		ctx := context.Background()
		days := cfg.AlertRetentionDays
		pool.Exec(ctx,
			`DELETE FROM alerts WHERE status = 'RESOLVED' AND resolved_at < NOW() - ($1 || ' days')::INTERVAL`, days)
		log.Println("Alert retention cleanup completed")
	}
}

func logRetentionWorker(pool *pgxpool.Pool, cfg *config.Config) {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		ctx := context.Background()
		days := cfg.LogRetentionDays
		pool.Exec(ctx,
			`DELETE FROM logs WHERE timestamp < NOW() - ($1 || ' days')::INTERVAL`, days)
		log.Println("Log retention cleanup completed")
	}
}

func notificationDeliveryWorker(pool *pgxpool.Pool) {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		ctx := context.Background()
		rows, err := pool.Query(ctx,
			`SELECT id, title, message FROM notification_deliveries WHERE status = 'QUEUED' ORDER BY created_at LIMIT 10`)
		if err != nil {
			log.Printf("Notification query error: %v", err)
			continue
		}
		count := 0
		for rows.Next() {
			var id, title, message string
			rows.Scan(&id, &title, &message)
			pool.Exec(ctx, `UPDATE notification_deliveries SET status = 'SENT', sent_at = NOW() WHERE id = $1`, id)
			count++
		}
		rows.Close()
		if count > 0 {
			log.Printf("Notification delivery: processed %d", count)
		}
	}
}
