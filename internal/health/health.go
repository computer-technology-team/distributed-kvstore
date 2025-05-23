package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// HealthStatus represents the health status response
type HealthStatus struct {
	Status string `json:"status"`
}

// Handler returns an HTTP handler for health checks
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := HealthStatus{
			Status: "ok",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(status)
		if err != nil {
			slog.Error("could not json encode respsone status", "error", err)
		}
	})
}

// AddHealthCheckEndpoint adds a health check endpoint to the provided mux
func AddHealthCheckEndpoint(mux *http.ServeMux) {
	mux.Handle("/health", Handler())
}
