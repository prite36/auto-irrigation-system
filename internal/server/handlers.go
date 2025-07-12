package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/scheduler"
)

// SlackEventsHandler creates a new http.HandlerFunc for handling Slack events.
// It verifies the request signature using the signing secret.
func SlackEventsHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, cfg.Slack.SigningSecret)
		if err != nil {
			log.Printf("[ERROR] Failed to create secrets verifier: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[ERROR] Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// We need to read the body twice, so we create a new reader with the same content.
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		if _, err := verifier.Write(body); err != nil {
			log.Printf("[ERROR] Failed to write body to verifier: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := verifier.Ensure(); err != nil {
			log.Printf("[WARN] Invalid Slack signature: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			log.Printf("[ERROR] Failed to parse Slack event: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if eventsAPIEvent.Type == slackevents.URLVerification {
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal(body, &r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(r.Challenge))
			log.Printf("[INFO] Responded to Slack URL verification challenge.")
			return
		}

		if eventsAPIEvent.Type == slackevents.CallbackEvent {
			// Here you can handle different callback events, like slash commands or messages
			log.Printf("[INFO] Received a callback event: %v", eventsAPIEvent.InnerEvent.Type)
			// For now, just acknowledge the event
			w.WriteHeader(http.StatusOK)
		}
	}
}

// TriggerJobHandler creates an http.HandlerFunc to manually trigger an irrigation job.
// TriggerTaskRequest is the request body for the TriggerTaskHandler
type TriggerTaskRequest struct {
	DeviceID string `json:"deviceId"`
}

// TriggerTaskHandler creates an http.HandlerFunc to manually trigger an irrigation task.
func TriggerTaskHandler(sched *scheduler.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var req TriggerTaskRequest
		// Decode the request body.
		if r.Body != nil && r.ContentLength > 0 {
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil && err != io.EOF {
				http.Error(w, "Error parsing request body", http.StatusBadRequest)
				return
			}
		}

		if req.DeviceID != "" {
			log.Printf("[INFO] Received API request to trigger task for device: %s", req.DeviceID)
			go func() {
				if err := sched.RunJobForDevice(req.DeviceID); err != nil {
					log.Printf("[ERROR] Failed to trigger job for device %s: %v", req.DeviceID, err)
				}
			}()
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintf(w, "Task trigger request for device %s accepted.", req.DeviceID)
		} else {
			log.Println("[INFO] Received API request to trigger all tasks.")
			go sched.RunAllJobsOnce()
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprintln(w, "Task trigger request for all devices accepted.")
		}
	}
}

func TriggerJobHandler(sched *scheduler.Scheduler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("[INFO] Received API request to trigger irrigation job manually.")
		// Run in a goroutine so we can respond to the client immediately
		go sched.RunAllJobsOnce()
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprintln(w, "Irrigation job trigger request accepted.")
	}
}
