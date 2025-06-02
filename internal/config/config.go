package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type MQTTConfig struct {
	Broker   string `env:"MQTT_BROKER" required:"true"`
	ClientID string `env:"MQTT_CLIENT_ID"`
	Username string `env:"MQTT_USERNAME"`
	Password string `env:"MQTT_PASSWORD"`
}

type DatabaseConfig struct {
	Host     string `env:"DB_HOST" required:"true"`
	Port     int    `env:"DB_PORT" required:"true"`
	User     string `env:"DB_USER" required:"true"`
	Password string `env:"DB_PASSWORD"`
	DBName   string `env:"DB_NAME" required:"true"`
	SSLMode  string `env:"DB_SSLMODE"`
}

type ScheduleConfig struct {
	Time     string `env:"SCHEDULE_TIME" required:"true"`     // Cron format (e.g., "0 6 * * *" for 6 AM daily)
	Duration int    `env:"SCHEDULE_DURATION" required:"true"` // Duration in minutes
}

type Config struct {
	MQTT     MQTTConfig
	Database DatabaseConfig
	Schedule ScheduleConfig
}

// envLoader is a helper to load environment variables into a struct
type envLoader struct {
	errors []error
}

// load loads environment variables into the provided struct based on tags
func (l *envLoader) load(cfg interface{}) error {
	val := reflect.ValueOf(cfg).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		envKey := strings.ToUpper(fieldType.Name)
		if envTag != "-" {
			envKey = envTag
		}

		envValue, exists := os.LookupEnv(envKey)
		if !exists || envValue == "" {
			if fieldType.Tag.Get("required") == "true" {
				l.errors = append(l.errors, fmt.Errorf("required environment variable %s is not set", envKey))
			}
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(envValue, 10, 64)
			if err != nil {
				l.errors = append(l.errors, fmt.Errorf("environment variable %s must be an integer: %v", envKey, err))
				continue
			}
			field.SetInt(intVal)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(envValue)
			if err != nil {
				l.errors = append(l.errors, fmt.Errorf("environment variable %s must be a boolean: %v", envKey, err))
				continue
			}
			field.SetBool(boolVal)
		}
	}

	if len(l.errors) > 0 {
		return fmt.Errorf("configuration error: %v", l.errors)
	}
	return nil
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	loader := &envLoader{}
	// Config{} - สร้าง instance ใหม่ของ struct Config
	// & - ดึง memory address ของ instance นั้น (pointer)
	cfg := &Config{}

	// Load MQTT config
	if err := loader.load(&cfg.MQTT); err != nil {
		return nil, err
	}

	// Load Database config
	if err := loader.load(&cfg.Database); err != nil {
		return nil, err
	}

	// Load Schedule config
	if err := loader.load(&cfg.Schedule); err != nil {
		return nil, err
	}

	// Check for any loading errors
	if len(loader.errors) > 0 {
		return nil, fmt.Errorf("configuration errors: %v", loader.errors)
	}

	return cfg, nil
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
