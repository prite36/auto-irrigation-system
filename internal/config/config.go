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
	Broker   string
	ClientID string
	Username string
	Password string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type ScheduleConfig struct {
	Times    string
	Duration int
}

type SlackConfig struct {
	BotToken      string
	ChannelID     string
	SigningSecret string
}

type DeviceConfig struct {
	ID      string   `json:"id"`
	TaskIDs []string `json:"taskIds"`
}

type Config struct {
	MQTT          MQTTConfig
	Database      DatabaseConfig
	Schedule      ScheduleConfig
	Slack         SlackConfig
	Devices       []DeviceConfig `json:"devices"`
	DeviceCfgPath string         `json:"devicecfgpath"`
}

func LoadConfig() (*Config, error) {
	log.Println("--- Starting Configuration Loading ---")
	v := viper.New()

	v.BindEnv("database.host", "DB_HOST")
	v.BindEnv("database.port", "DB_PORT")
	v.BindEnv("database.user", "DB_USER")
	v.BindEnv("database.password", "DB_PASSWORD")
	v.BindEnv("database.dbname", "DB_NAME")
	v.BindEnv("database.sslmode", "DB_SSLMODE")

	v.BindEnv("mqtt.broker", "MQTT_BROKER")
	v.BindEnv("mqtt.clientid", "MQTT_CLIENT_ID")
	v.BindEnv("mqtt.username", "MQTT_USERNAME")
	v.BindEnv("mqtt.password", "MQTT_PASSWORD")

	v.BindEnv("schedule.times", "SCHEDULE_TIMES")
	v.BindEnv("schedule.duration", "SCHEDULE_DURATION")

	v.BindEnv("slack.bottoken", "SLACK_BOT_TOKEN")
	v.BindEnv("slack.channelid", "SLACK_CHANNEL_ID")
	v.BindEnv("slack.signingsecret", "SLACK_SIGNING_SECRET")

	v.BindEnv("devicecfgpath", "DEVICE_CONFIG_PATH")

	log.Println("[1] Explicit environment variable binding configured.")

	env := os.Getenv("APP_ENV")
	if env == "" {
		log.Println("[2] APP_ENV not set, defaulting to 'local'.")
		env = "local"
	} else {
		log.Printf("[2] APP_ENV is set to '%s'.", env)
	}

	if env == "local" {
		log.Println("[3] Attempting to load .env.local file...")
		v.SetConfigFile(".env.local")
		v.SetConfigType("env")

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				log.Printf("Error: Failed to read config file .env.local: %v", err)
				return nil, fmt.Errorf("error reading config file .env.local: %w", err)
			}
			log.Println("Info: .env.local not found, which is acceptable. Relying on environment variables.")
		} else {
			log.Printf("Success: Loaded configuration from %s", v.ConfigFileUsed())
		}
	} else {
		log.Printf("[3] Skipping .env file loading because APP_ENV is '%s'.", env)
	}

	log.Println("[4] Dumping all settings found by Viper (sensitive info redacted):")

	var config Config
	log.Println("[5] Unmarshaling settings into Config struct...")
	if err := v.Unmarshal(&config); err != nil {
		log.Printf("Error: Failed to unmarshal config: %v", err)
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	log.Println("[6] Final configuration struct (sensitive info redacted):")

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
