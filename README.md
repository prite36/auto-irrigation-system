# Auto Irrigation System

A Go-based irrigation system that controls IOT sprinklers via MQTT with a daily schedule.

## Features

- Schedule-based sprinkler control using cron expressions
- MQTT integration for IOT device control
- SQLite database for storing irrigation history
- Configurable schedule and duration

## Prerequisites

- Go 1.21 or later
- MQTT broker (e.g., Mosquitto, EMQX)
- IOT device that can be controlled via MQTT

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/prite36/auto-irrigation-system.git
   cd auto-irrigation-system
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o irrigation ./cmd/irrigation
   ```

## Configuration

The application uses environment variables for configuration. Copy the `.env.example` to `.env` and update the values:

```bash
cp .env.example .env
```

### Environment Variables

#### MQTT Configuration
- `MQTT_BROKER`: MQTT broker URL (default: `tcp://localhost:1883`)
- `MQTT_CLIENT_ID`: Client ID for MQTT connection (default: `irrigation-system`)
- `MQTT_USERNAME`: MQTT username (optional)
- `MQTT_PASSWORD`: MQTT password (optional)

#### Database Configuration
- `DB_HOST`: PostgreSQL host (default: `localhost`)
- `DB_PORT`: PostgreSQL port (default: `5432`)
- `DB_USER`: PostgreSQL user (default: `postgres`)
- `DB_PASSWORD`: PostgreSQL password
- `DB_NAME`: Database name (default: `irrigation`)
- `DB_SSLMODE`: SSL mode (default: `disable`)

#### Schedule Configuration
- `SCHEDULE_TIME`: Cron expression for scheduling (default: `0 6 * * *` for 6 AM daily)
- `SCHEDULE_DURATION`: Duration in minutes (default: `10`)

## Usage

1. Start your MQTT broker if it's not already running.
2. Run the application:
   ```bash
   ./irrigation
   ```

3. The application will start and begin monitoring the schedule.

## MQTT Topics

- `sprinkler/control`: Used to control the sprinkler
  - Publish `on` to turn on the sprinkler
  - Publish `off` to turn off the sprinkler

## Database

The application uses SQLite to store irrigation history. The database file is created automatically at `irrigation.db`.

## Project Structure

```
.
├── cmd/
│   └── irrigation/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/            # Configuration structures
│   ├── mqtt/               # MQTT client implementation
│   ├── models/             # Database models
│   ├── scheduler/          # Scheduling logic
│   └── service/            # Application service layer
├── go.mod
└── README.md
```

## Development

### Running Tests

```bash
go test ./...
```

### Building for Production

```bash
go build -ldflags="-s -w" -o irrigation ./cmd/irrigation
```

## License

MIT
