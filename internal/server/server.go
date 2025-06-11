package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/scheduler"
)

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

	addr := ":3005" // You can make this configurable
	log.Printf("API Server configured to listen on %s", addr)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
