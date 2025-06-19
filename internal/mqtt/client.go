package mqtt

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prite36/auto-irrigation-system/internal/config"
	"github.com/prite36/auto-irrigation-system/internal/models"
)

// Client manages the MQTT connection and subscriptions.
type Client struct {
	client            mqtt.Client
	deviceStatuses    sync.Map // Maps deviceID (string) to *models.DeviceStatus
	subscribedDevices sync.Map // To track which devices we are subscribed to (key: deviceID, value: config.DeviceConfig)
}

// NewClient creates and configures a new MQTT client.
func NewClient(broker, clientID, username, password string) (*Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectTimeout(30 * time.Second)

	c := &Client{}
	opts.SetDefaultPublishHandler(c.messageHandler)
	opts.SetOnConnectHandler(c.onConnectHandler)
	opts.SetConnectionLostHandler(c.connectionLostHandler)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	c.client = client
	return c, nil
}

// onConnectHandler is called when the client connects or reconnects.
func (c *Client) onConnectHandler(client mqtt.Client) {
	log.Println("Connected to MQTT broker.")
	// Re-subscribe to topics for all previously subscribed devices
	c.subscribedDevices.Range(func(key, value interface{}) bool {
		device := value.(config.DeviceConfig)
		log.Printf("Re-subscribing to topics for device: %s", device.ID)
		c.SubscribeToDeviceTopics(device)
		return true
	})
}

// connectionLostHandler is called when the connection is lost.
func (c *Client) connectionLostHandler(client mqtt.Client, err error) {
	log.Printf("Connection to MQTT broker lost: %v", err)
}

// messageHandler processes incoming MQTT messages.
func (c *Client) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message on topic: %s with payload: %s", msg.Topic(), msg.Payload())

	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 3 {
		log.Printf("Warning: Received message on unexpected topic format: %s", msg.Topic())
		return
	}
	deviceID := parts[0]
	payloadStr := string(msg.Payload())

	// Get or create the status object for the device. IMPORTANT: Store POINTERS in the map.
	value, _ := c.deviceStatuses.LoadOrStore(deviceID, &models.DeviceStatus{DeviceID: deviceID})
	status := value.(*models.DeviceStatus)

	var err error
	switch {
	case strings.HasSuffix(msg.Topic(), "/status/health_check"):
		status.HealthCheck, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/sprinkler/position"):
		status.SprinklerPosition, err = strconv.ParseFloat(payloadStr, 64)
	case strings.HasSuffix(msg.Topic(), "/status/valve/position"):
		status.ValvePosition, err = strconv.ParseFloat(payloadStr, 64)
	case strings.HasSuffix(msg.Topic(), "/status/sprinkler/calib_complete"):
		status.SprinklerCalibComplete, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/valve/calib_complete"):
		status.ValveCalibComplete, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/valve/target"):
		status.ValveIsAtTarget, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/current_index"):
		status.TaskCurrentIndex, err = strconv.Atoi(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/current_count"):
		status.TaskCurrentCount, err = strconv.Atoi(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/all_complete"):
		status.TaskAllComplete, err = strconv.ParseBool(payloadStr)
	case strings.HasSuffix(msg.Topic(), "/status/task/array"):
		status.TaskArray = payloadStr
	default:
		log.Printf("Warning: No handler for topic: %s", msg.Topic())
		return // No need to store status again if topic is unknown
	}

	if err != nil {
		log.Printf("Error parsing payload for topic %s: %v", msg.Topic(), err)
		return
	}

	// No need to store back, as we are modifying the pointer.
}

// Publish sends a message to a given topic.
func (c *Client) Publish(topic, payload string) {
	if token := c.client.Publish(topic, 1, false, payload); token.Wait() && token.Error() != nil {
		log.Printf("Failed to publish to topic %s: %v", topic, token.Error())
	}
}

// Close disconnects the MQTT client.
func (c *Client) Close() {
	c.client.Disconnect(250)
	log.Println("MQTT client disconnected.")
}

// SubscribeToDeviceTopics subscribes to all relevant status topics for a given device.
func (c *Client) SubscribeToDeviceTopics(device config.DeviceConfig) {
	// Mark this device as one we want to be subscribed to, for reconnections.
	c.subscribedDevices.Store(device.ID, device)

	var topics map[string]byte

	switch device.Type {
	case "iot_sprinkler":
		topics = map[string]byte{
			fmt.Sprintf("%s/status/sprinkler/position", device.ID):       0,
			fmt.Sprintf("%s/status/valve/position", device.ID):           0,
			fmt.Sprintf("%s/status/sprinkler/calib_complete", device.ID): 0,
			fmt.Sprintf("%s/status/valve/calib_complete", device.ID):     0,
			fmt.Sprintf("%s/status/valve/target", device.ID):             0,
			fmt.Sprintf("%s/status/task/current_index", device.ID):       0,
			fmt.Sprintf("%s/status/task/current_count", device.ID):       0,
			fmt.Sprintf("%s/status/task/all_complete", device.ID):        0,
			fmt.Sprintf("%s/status/task/array", device.ID):               0,
		}
	case "iot_plant_pot":
		topics = map[string]byte{
			fmt.Sprintf("%s/status/health_check", device.ID): 0,
		}
	default:
		log.Printf("Warning: Unknown device type '%s' for device '%s'. No topics will be subscribed.", device.Type, device.ID)
		return
	}

	for topic := range topics {
		if token := c.client.Subscribe(topic, 1, nil); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to topic %s: %v", topic, token.Error())
		} else {
			log.Printf("Subscribed to topic: %s", topic)
		}
	}
}

// GetDeviceStatus safely retrieves the status for a given device ID.
func (c *Client) GetDeviceStatus(deviceID string) *models.DeviceStatus {
	value, ok := c.deviceStatuses.Load(deviceID)
	if !ok {
		return &models.DeviceStatus{DeviceID: deviceID} // Return a new empty status to avoid nil pointers
	}
	return value.(*models.DeviceStatus)
}

// ResetDeviceStatus resets the status for a device, typically before a new operation.
func (c *Client) ResetDeviceStatus(deviceID string) {
	log.Printf("Resetting status for device %s", deviceID)
	c.deviceStatuses.Store(deviceID, &models.DeviceStatus{DeviceID: deviceID})
}
