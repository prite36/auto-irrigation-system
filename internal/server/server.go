package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/scheduler"
)

type StatusResponse struct {
	Environment string `json:"environment"`
	Status      string `json:"status"`
}

// New creates a new HTTP server and sets up the routes.
func New(cfg *config.Config, sched *scheduler.Scheduler) *http.Server {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	// Slack events endpoint
	mux.HandleFunc("/slack/events", SlackEventsHandler(cfg))

	// API endpoint to trigger a job manually
	mux.HandleFunc("/api/v1/irrigate/now", TriggerJobHandler(sched))

	// API endpoint to get application status
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		env := os.Getenv("APP_ENV")
		if env == "" {
			env = "development"
		}

		response := StatusResponse{
			Environment: env,
			Status:      "ok",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	addr := ":3005" // You can make this configurable
	log.Printf("API Server configured to listen on %s", addr)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
