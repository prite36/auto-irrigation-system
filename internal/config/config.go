package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/viper"
)

type MQTTConfig struct {
	Broker   string `mapstructure:"MQTT_BROKER"`
	ClientID string `mapstructure:"MQTT_CLIENT_ID"`
	Username string `mapstructure:"MQTT_USERNAME"`
	Password string `mapstructure:"MQTT_PASSWORD"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"DB_HOST"`
	Port     int    `mapstructure:"DB_PORT"`
	User     string `mapstructure:"DB_USER"`
	Password string `mapstructure:"DB_PASSWORD"`
	DBName   string `mapstructure:"DB_NAME"`
	SSLMode  string `mapstructure:"DB_SSLMODE"`
}

type ScheduleConfig struct {
	Times    string `mapstructure:"SCHEDULE_TIMES"` // Comma-separated e.g., "07:00,17:00"
	Duration int    `mapstructure:"SCHEDULE_DURATION"`
}

// SlackConfig holds the configuration for Slack notifications.
type SlackConfig struct {
	BotToken      string `mapstructure:"SLACK_BOT_TOKEN"`
	ChannelID     string `mapstructure:"SLACK_CHANNEL_ID"`
	SigningSecret string `mapstructure:"SLACK_SIGNING_SECRET"`
}

// DeviceConfig defines a single sprinkler device and its associated task IDs.
type DeviceConfig struct {
	ID      string   `json:"id"`
	TaskIDs []string `json:"taskIds"`
}

type Config struct {
	MQTT     MQTTConfig     `mapstructure:",squash"`
	Database DatabaseConfig `mapstructure:",squash"`
	Schedule ScheduleConfig `mapstructure:",squash"`
	Slack    SlackConfig    `mapstructure:",squash"` // Added Slack configuration
	Devices  []DeviceConfig `json:"devices"`
	DeviceCfgPath string         `mapstructure:"DEVICE_CONFIG_PATH"`
}

// LoadConfig reads configuration from a .env file and environment variables,
// and also loads a separate device configuration JSON file.
func LoadConfig() (*Config, error) {
	// Configure viper for .env file and environment variables
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv() // Read in environment variables that match

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			log.Println("Warning: .env file not found, relying on environment variables.")
		} else {
			// Config file was found but another error was produced
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct, %v", err)
	}

	// Load device configurations from the specified JSON file
	if config.DeviceCfgPath != "" {
		jsonFile, err := os.Open(config.DeviceCfgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open device config file '%s': %w", config.DeviceCfgPath, err)
		}
		defer jsonFile.Close()

		byteValue, err := io.ReadAll(jsonFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read device config file: %w", err)
		}

		// The JSON structure should be an object with a "devices" key, e.g. { "devices": [ ... ] }
		// We unmarshal into the config struct which has the `json:"devices"` tag on the Devices field.
		if err := json.Unmarshal(byteValue, &config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal device config JSON: %w", err)
		}
	}

	return &config, nil
}

// DefaultConfig is kept for backward compatibility but will be removed in the future
// Use LoadConfig instead
func DefaultConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	return cfg
}

// DSN returns the PostgreSQL connection string
func (cfg *Config) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)
}
