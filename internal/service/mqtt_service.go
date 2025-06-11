package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/eclipse/paho.mqtt.golang" 
	"github.com/prite36/auto-irrigation-system/internal/models"
)

// MQTTService handles MQTT client connections and subscriptions.
// It stores the status of all sprinkler devices.
type MQTTService struct {
	Client         mqtt.Client
	DeviceStatuses sync.Map // Concurrent map to store SprinklerStatus by device ID
}

// NewMQTTService creates and configures a new MQTT service.
func NewMQTTService(broker, clientID, username, password string) (*MQTTService, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)

	service := &MQTTService{}

	opts.SetDefaultPublishHandler(service.messageHandler)
	opts.OnConnect = service.connectHandler
	opts.OnConnectionLost = service.connectionLostHandler

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	service.Client = client
	return service, nil
}

// SubscribeToDeviceTopics subscribes to all relevant topics for a given device ID.
func (s *MQTTService) SubscribeToDeviceTopics(deviceID string) {
	topics := []string{
		fmt.Sprintf("%s/waterValve/status/isAtTargetPosition", deviceID),
		fmt.Sprintf("%s/waterValve/status/calibrationComplete", deviceID),
		fmt.Sprintf("%s/sprinkler/status/calibrationComplete", deviceID),
		fmt.Sprintf("%s/sprinkler/status/tasksArray", deviceID),
		fmt.Sprintf("%s/sprinkler/status/allTasksComplete", deviceID),
	}

	for _, topic := range topics {
		if token := s.Client.Subscribe(topic, 1, nil); token.Wait() && token.Error() != nil {
			log.Printf("Failed to subscribe to topic %s: %v", topic, token.Error())
		}
		log.Printf("Subscribed to topic: %s", topic)
	}
}

// messageHandler is the default handler for incoming messages.
func (s *MQTTService) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())

	topicParts := strings.Split(msg.Topic(), "/")
	if len(topicParts) < 2 {
		log.Printf("Ignoring message from unexpected topic: %s", msg.Topic())
		return
	}
	deviceID := topicParts[0]

	// Get or create the status object for the device
	value, _ := s.DeviceStatuses.LoadOrStore(deviceID, &models.SprinklerStatus{DeviceID: deviceID})
	status := value.(*models.SprinklerStatus)

	// Update status based on topic
	switch {
	case strings.HasSuffix(msg.Topic(), "/isAtTargetPosition"):
		status.IsAtTargetPosition = string(msg.Payload()) == "true"
	case strings.HasSuffix(msg.Topic(), "/waterValve/status/calibrationComplete"):
		status.WaterValveCalibrationComplete = string(msg.Payload()) == "true"
	case strings.HasSuffix(msg.Topic(), "/sprinkler/status/calibrationComplete"):
		status.SprinklerCalibrationComplete = string(msg.Payload()) == "true"
	case strings.HasSuffix(msg.Topic(), "/tasksArray"):
		status.TasksArray = string(msg.Payload())
		case strings.HasSuffix(msg.Topic(), "/allTasksComplete"):
		status.AllTasksComplete = string(msg.Payload()) == "true"
	}

	s.DeviceStatuses.Store(deviceID, status)

	// For debugging: print the current state
	currentState, _ := json.Marshal(status)
	log.Printf("Updated status for %s: %s", deviceID, string(currentState))
}

// connectHandler is called upon a successful connection.
func (s *MQTTService) connectHandler(client mqtt.Client) {
	log.Println("Connected to MQTT broker")
}

// connectionLostHandler is called when the connection is lost.
func (s *MQTTService) connectionLostHandler(client mqtt.Client, err error) {
	log.Printf("Connection to MQTT broker lost: %v", err)
}

// GetDeviceStatus retrieves the status of a specific device.
func (s *MQTTService) GetDeviceStatus(deviceID string) (*models.SprinklerStatus, bool) {
	value, ok := s.DeviceStatuses.Load(deviceID)
	if !ok {
		return nil, false
	}
	return value.(*models.SprinklerStatus), true
}
