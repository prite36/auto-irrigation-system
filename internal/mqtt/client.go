package mqtt

import (
	"fmt"
	"log"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client mqtt.Client
	topic  string
}

func NewClient(broker, clientID, username, password string) (*Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return &Client{
		client: client,
		topic:  "sprinkler/control",
	}, nil
}

func (c *Client) PublishSprinklerControl(state bool) error {
	payload := "off"
	if state {
		payload = "on"
	}

	token := c.client.Publish(c.topic, 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout publishing to topic %s", c.topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("error publishing to topic %s: %w", c.topic, token.Error())
	}

	log.Printf("Published sprinkler %s to %s\n", payload, c.topic)
	return nil
}

func (c *Client) Close() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}
