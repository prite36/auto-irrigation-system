package slack

import (
	"log"

	"github.com/slack-go/slack"
)

// Client wraps the slack client
type Client struct {
	api       *slack.Client
	channelID string
}

// NewClient creates a new slack client
func NewClient(token, channelID string) *Client {
	if token == "" || channelID == "" {
		log.Println("Slack token or channel ID is not configured. Slack notifications will be disabled.")
		return nil // Return nil if not configured
	}
	api := slack.New(token)
	return &Client{
		api:       api,
		channelID: channelID,
	}
}

// SendMessage sends a simple text message, now wrapped as an info block.
func (c *Client) SendMessage(message string) {
	if c == nil || c.api == nil {
		return // Do nothing if client is not initialized
	}
	c.SendRichMessage(NewInfoMessage("Scheduler Notification", message))
}

// SendRichMessage sends a message using block kit options.
func (c *Client) SendRichMessage(options slack.MsgOption) {
	if c == nil || c.api == nil {
		return // Do nothing if client is not initialized
	}

	_, _, err := c.api.PostMessage(c.channelID, options)
	if err != nil {
		log.Printf("Failed to send rich Slack message: %v", err)
	}
}
