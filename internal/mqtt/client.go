package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/prite36/auto-irrigation-system/internal/models"
)

// Client handles MQTT connections, subscriptions, and stores device statuses.
type Client struct {
	client         mqtt.Client
	DeviceStatuses sync.Map // Concurrent map to store SprinklerStatus by device ID
}

// NewClient creates and configures a new MQTT Client.
func NewClient(broker, clientID, username, password string) (*Client, error) {
	c := &Client{}

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
	opts.SetDefaultPublishHandler(c.messageHandler)
	opts.OnConnect = c.connectHandler
	opts.OnConnectionLost = c.connectionLostHandler

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	c.client = client
	return c, nil
}

// connectHandler is called upon a successful connection.
func (c *Client) connectHandler(client mqtt.Client) {
	log.Println("Connected to MQTT broker")
	// Note: If you have a dynamic list of devices, you might need to resubscribe here.
	// For a fixed list initialized at startup, this is fine.
}

// connectionLostHandler is called when the connection is lost.
func (c *Client) connectionLostHandler(client mqtt.Client, err error) {
	log.Printf("Connection to MQTT broker lost: %v", err)
}

// messageHandler is the default handler for incoming messages.
func (c *Client) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: '%s' from topic: %s\n", msg.Payload(), msg.Topic())

	topicParts := strings.Split(msg.Topic(), "/")
	if len(topicParts) < 2 {
		log.Printf("Ignoring message from unexpected topic: %s", msg.Topic())
		return
	}
	deviceID := topicParts[0]

	// Get or create the status object for the device
	value, _ := c.DeviceStatuses.LoadOrStore(deviceID, &models.SprinklerStatus{DeviceID: deviceID})
	status := value.(*models.SprinklerStatus)

	payloadStr := string(msg.Payload())

	// Update status based on topic
	switch {
	case strings.HasSuffix(msg.Topic(), "/isAtTargetPosition"):
		status.IsAtTargetPosition = payloadStr == "true"
	case strings.HasSuffix(msg.Topic(), "/waterValve/status/calibrationComplete"):
		status.WaterValveCalibrationComplete = payloadStr == "true"
	case strings.HasSuffix(msg.Topic(), "/sprinkler/status/calibrationComplete"):
		status.SprinklerCalibrationComplete = payloadStr == "true"
	case strings.HasSuffix(msg.Topic(), "/tasksArray"):
		status.TasksArray = payloadStr
	case strings.HasSuffix(msg.Topic(), "/allTasksComplete"):
		status.AllTasksComplete = payloadStr == "true"
	default:
		log.Printf("No handler for topic: %s", msg.Topic())
		return
	}

	c.DeviceStatuses.Store(deviceID, status)

	// For debugging: print the current state
	currentState, _ := json.Marshal(status)
	log.Printf("Updated status for %s: %s", deviceID, string(currentState))
}

// SubscribeToDeviceTopics subscribes to all relevant topics for a given device ID.
func (c *Client) SubscribeToDeviceTopics(deviceID string) {
	topics := map[string]byte{
		fmt.Sprintf("%s/waterValve/status/isAtTargetPosition", deviceID):    1,
		fmt.Sprintf("%s/waterValve/status/calibrationComplete", deviceID):   1,
		fmt.Sprintf("%s/sprinkler/status/calibrationComplete", deviceID):  1,
		fmt.Sprintf("%s/sprinkler/status/tasksArray", deviceID):             1,
		fmt.Sprintf("%s/sprinkler/status/allTasksComplete", deviceID):      1,
	}

	if token := c.client.SubscribeMultiple(topics, nil); token.Wait() && token.Error() != nil {
		log.Printf("Failed to subscribe to topics for device %s: %v", deviceID, token.Error())
		return
	}

	log.Printf("Subscribed to all topics for device: %s", deviceID)
}

// GetDeviceStatus retrieves the status of a specific device.
func (c *Client) GetDeviceStatus(deviceID string) (*models.SprinklerStatus, bool) {
	value, ok := c.DeviceStatuses.Load(deviceID)
	if !ok {
		return nil, false
	}
	return value.(*models.SprinklerStatus), true
}

// PublishSprinklerControl sends a command to turn a sprinkler on or off.
func (c *Client) PublishSprinklerControl(deviceID string, state bool) error {
	payload := "off"
	if state {
		payload = "on"
	}
	topic := fmt.Sprintf("%s/sprinkler/control/turn", deviceID)

	token := c.client.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout publishing to topic %s", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("error publishing to topic %s: %w", topic, token.Error())
	}

	log.Printf("Published '%s' to topic '%s'\n", payload, topic)
	return nil
}

// Close disconnects the MQTT client.
func (c *Client) Close() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}
