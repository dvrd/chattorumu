package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"jobsity-chat/internal/messaging"
)

// Health returns basic health check
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status    string                 `json:"status"`
	LatencyMs int64                  `json:"latency_ms,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Ready returns readiness check with dependencies
func Ready(db *sql.DB, rmq *messaging.RabbitMQ) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check dependencies in parallel
		dbResult := make(chan HealthCheckResult, 1)
		rmqResult := make(chan HealthCheckResult, 1)

		go func() {
			dbResult <- checkDatabase(ctx, db)
		}()

		go func() {
			rmqResult <- checkRabbitMQ(ctx, rmq)
		}()

		dbCheck := <-dbResult
		rmqCheck := <-rmqResult

		response := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"checks": map[string]HealthCheckResult{
				"database": dbCheck,
				"rabbitmq": rmqCheck,
			},
		}

		allHealthy := dbCheck.Status == "up" && rmqCheck.Status == "up"

		if allHealthy {
			response["status"] = "ready"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		} else {
			response["status"] = "not_ready"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(response)
	}
}

// checkDatabase verifies database connectivity
func checkDatabase(ctx context.Context, db *sql.DB) HealthCheckResult {
	start := time.Now()
	err := db.PingContext(ctx)
	latency := time.Since(start)

	stats := db.Stats()

	if err != nil {
		return HealthCheckResult{
			Status:    "down",
			LatencyMs: latency.Milliseconds(),
			Error:     err.Error(),
		}
	}

	return HealthCheckResult{
		Status:    "up",
		LatencyMs: latency.Milliseconds(),
		Metadata: map[string]interface{}{
			"connections_open":   stats.OpenConnections,
			"connections_in_use": stats.InUse,
			"connections_idle":   stats.Idle,
			"max_open":           stats.MaxOpenConnections,
		},
	}
}

// checkRabbitMQ verifies RabbitMQ connectivity
func checkRabbitMQ(ctx context.Context, rmq *messaging.RabbitMQ) HealthCheckResult {
	start := time.Now()

	if rmq.IsClosed() {
		return HealthCheckResult{
			Status: "down",
			Error:  "connection closed",
		}
	}

	latency := time.Since(start)

	return HealthCheckResult{
		Status:    "up",
		LatencyMs: latency.Milliseconds(),
	}
}
