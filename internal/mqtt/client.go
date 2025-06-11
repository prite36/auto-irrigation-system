package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prite36/auto-irrigation-system/internal/models"
)

// Client handles MQTT connections, subscriptions, and stores device statuses.
type Client struct {
	client            mqtt.Client
	DeviceStatuses    *sync.Map
	subscribedDevices *sync.Map // To keep track of devices to re-subscribe on reconnect
}

// NewClient creates and configures a new MQTT Client.
func NewClient(broker, clientID, username, password string) (*Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)

	c := &Client{
		DeviceStatuses:    &sync.Map{},
		subscribedDevices: &sync.Map{},
	}

	opts.SetDefaultPublishHandler(c.messageHandler)
	opts.OnConnect = c.onConnectHandler
	opts.OnConnectionLost = c.connectionLostHandler

	client := mqtt.NewClient(opts)
	c.client = client
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return c, nil
}

// onConnectHandler is called upon a successful connection.
func (c *Client) onConnectHandler(client mqtt.Client) {
	log.Println("Connected to MQTT broker")
	// Re-subscribe to all known devices
	c.subscribedDevices.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		log.Printf("Re-subscribing to topics for device: %s", deviceID)
		c.SubscribeToDeviceTopics(deviceID)
		return true
	})
}

// connectionLostHandler is called when the connection is lost.
func (c *Client) connectionLostHandler(client mqtt.Client, err error) {
	log.Printf("Connection to MQTT broker lost: %v", err)
}

// messageHandler is the default handler for incoming messages.
func (c *Client) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: '%s' from topic: %s\n", msg.Payload(), msg.Topic())

	topicParts := strings.Split(msg.Topic(), "/")
	if len(topicParts) < 3 { // e.g., <deviceID>/status/sprinkler/position
		log.Printf("Ignoring message from unexpected topic format: %s", msg.Topic())
		return
	}
	deviceID := topicParts[0]

	// Load or create the status for the device
	value, _ := c.DeviceStatuses.LoadOrStore(deviceID, &models.SprinklerStatus{DeviceID: deviceID})
	status := value.(*models.SprinklerStatus)

	payloadStr := string(msg.Payload())

	var err error
	// Update status based on topic
	switch {
	case strings.HasSuffix(msg.Topic(), "/status/sprinkler/position"):
		status.SprinklerPosition, err = strconv.ParseFloat(payloadStr, 64)
	case strings.HasSuffix(msg.Topic(), "/status/valve/position"):
		status.ValvePosition, err = strconv.ParseFloat(payloadStr, 64)
	case strings.HasSuffix(msg.Topic(), "/status/sprinkler/calib_complete"):
		status.SprinklerCalibComplete, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/valve/calib_complete"):
		status.ValveCalibComplete, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/valve/is_at_target"):
		status.ValveIsAtTarget, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/current_index"):
		status.TaskCurrentIndex, err = strconv.Atoi(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/current_count"):
		status.TaskCurrentCount, err = strconv.Atoi(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/array"):
		status.TaskArray = payloadStr
	default:
		log.Printf("Unhandled topic: %s", msg.Topic())
		return
	}

	if err != nil {
		log.Printf("Failed to parse value '%s' for topic %s: %v", payloadStr, msg.Topic(), err)
		return
	}

	// Store the updated status back into the map
	c.DeviceStatuses.Store(deviceID, status)

	// For debugging: print the current state
	currentState, jsonErr := json.Marshal(status)
	if jsonErr != nil {
		log.Printf("Error marshalling status to JSON for device %s: %v", deviceID, jsonErr)
		return
	}
	log.Printf("Updated status for %s: %s", deviceID, string(currentState))
}

// SubscribeToDeviceTopics subscribes to all relevant topics for a given device ID.
func (c *Client) SubscribeToDeviceTopics(deviceID string) {
	// Store the device ID for re-subscription on reconnect
	c.subscribedDevices.Store(deviceID, true)

	topics := map[string]byte{
		fmt.Sprintf("%s/status/sprinkler/position", deviceID):       1,
		fmt.Sprintf("%s/status/valve/position", deviceID):           1,
		fmt.Sprintf("%s/status/sprinkler/calib_complete", deviceID): 1,
		fmt.Sprintf("%s/status/valve/calib_complete", deviceID):     1,
		fmt.Sprintf("%s/status/valve/is_at_target", deviceID):       1,
		fmt.Sprintf("%s/status/task/current_index", deviceID):       1,
		fmt.Sprintf("%s/status/task/current_count", deviceID):       1,
		fmt.Sprintf("%s/status/task/array", deviceID):               1,
	}

	if c.client.IsConnected() {
		if token := c.client.SubscribeMultiple(topics, nil); token.Wait() && token.Error() != nil {
			log.Printf("Error subscribing to topics for %s: %v", deviceID, token.Error())
			return
		}
		log.Printf("Subscribed to all topics for device: %s", deviceID)
	}
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
