package slack

import (
	"log"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// Client wraps the slack client
type Client struct {
	api       *slack.Client
	channelID string
	rateLimitBackoff time.Duration
}

// NewClient creates a new slack client
func NewClient(token, channelID string) *Client {
	if token == "" || channelID == "" {
		log.Println("Slack token or channel ID is not configured. Slack notifications will be disabled.")
		return nil // Return nil if not configured
	}
	api := slack.New(token)
	return &Client{
		api:              api,
		channelID:        channelID,
		rateLimitBackoff: 0,
	}
}

// SendMessage sends a simple text message, now wrapped as an info block.
func (c *Client) SendMessage(message string) {
	if c == nil || c.api == nil {
		return // Do nothing if client is not initialized
	}
	c.SendRichMessage(NewInfoMessage("Scheduler Notification", message))
}

// SendRichMessage sends a message using block kit options with rate limit handling.
func (c *Client) SendRichMessage(options slack.MsgOption) {
	if c == nil || c.api == nil {
		return // Do nothing if client is not initialized
	}

	// Check if we're in a backoff period
	if c.rateLimitBackoff > 0 {
		if time.Now().Before(time.Now().Add(-c.rateLimitBackoff)) {
			log.Printf("Skipping Slack message due to rate limit backoff (remaining: %v)", c.rateLimitBackoff)
			return
		}
		// Reset backoff if enough time has passed
		c.rateLimitBackoff = 0
	}

	_, _, err := c.api.PostMessage(c.channelID, options)
	if err != nil {
		if c.isRateLimitError(err) {
			c.handleRateLimit(err)
		} else {
			log.Printf("Failed to send rich Slack message: %v", err)
		}
	}
}

// isRateLimitError checks if the error is related to rate limiting
func (c *Client) isRateLimitError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate_limited") || 
		   strings.Contains(errStr, "message_limit_exceeded") ||
		   strings.Contains(errStr, "too_many_requests")
}

// handleRateLimit implements exponential backoff for rate limit errors
func (c *Client) handleRateLimit(err error) {
	// Start with 1 minute backoff, can be extended based on error type
	backoffDuration := 1 * time.Minute
	
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "message_limit_exceeded") {
		// For message limit exceeded, use longer backoff
		backoffDuration = 5 * time.Minute
	}
	
	c.rateLimitBackoff = backoffDuration
	log.Printf("Slack rate limit detected (%v). Messages will be suppressed for %v", err, backoffDuration)
	
	// Schedule backoff reset
	go func() {
		time.Sleep(backoffDuration)
		c.rateLimitBackoff = 0
		log.Println("Slack rate limit backoff period ended. Messages will resume.")
	}()
}

// IsRateLimited returns true if the client is currently in a rate limit backoff period
func (c *Client) IsRateLimited() bool {
	if c == nil {
		return false
	}
	return c.rateLimitBackoff > 0
}

// SendMessageSafe sends a message only if not rate limited, returns true if sent
func (c *Client) SendMessageSafe(message string) bool {
	if c == nil || c.IsRateLimited() {
		return false
	}
	c.SendMessage(message)
	return true
}

// SendRichMessageSafe sends a rich message only if not rate limited, returns true if sent
func (c *Client) SendRichMessageSafe(options slack.MsgOption) bool {
	if c == nil || c.IsRateLimited() {
		return false
	}
	c.SendRichMessage(options)
	return true
}