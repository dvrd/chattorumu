package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// Health returns basic health check
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// Ready returns readiness check with dependencies
func Ready(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]string{
			"status": "ready",
		}

		// Check database
		if err := db.Ping(); err != nil {
			response["database"] = "disconnected"
			response["status"] = "not_ready"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(response)
			return
		}
		response["database"] = "connected"

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
