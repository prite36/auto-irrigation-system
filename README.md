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

#### Slack Configuration
- `SLACK_BOT_TOKEN`: Your Slack bot token (for sending notifications).
- `SLACK_CHANNEL_ID`: The ID of the Slack channel to send notifications to.
- `SLACK_SIGNING_SECRET`: Your Slack app's signing secret (for verifying incoming events).

## Local Development

### Prerequisites

1. Go (version 1.20 or later)
2. PostgreSQL (version 15 or later)
3. MQTT Broker (e.g., Mosquitto)

### Setup

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/auto-irrigation-system.git
    cd auto-irrigation-system
    ```

2.  **Configure Environment:**
    Create a `.env` file in the root directory and populate it with your configuration. See the [Configuration](#configuration) section for details.

3.  **Install Dependencies:**
    ```bash
    go mod tidy
    ```

4.  **Run the Application:**
    The main application now runs both the background scheduler and the API server in a single process.
    ```bash
    go run ./cmd/irrigation
    ```

    **Run for debug
    ```bash
    go run ./cmd/debug/main.go
    ```

## Configuration

The application is configured using environment variables. Create a `.env` file in the project root or set these variables in your shell.

| Variable              | Description                                      | Example                                  |
| --------------------- | ------------------------------------------------ | ---------------------------------------- |
| `MQTT_BROKER`         | URL of the MQTT broker.                          | `tcp://localhost:1883`                   |
| `MQTT_CLIENT_ID`      | Unique client ID for the application.            | `auto-irrigation-system`                 |
| `MQTT_USERNAME`       | (Optional) Username for MQTT authentication.     | `myuser`                                 |
| `MQTT_PASSWORD`       | (Optional) Password for MQTT authentication.     | `mypassword`                             |
| `DB_HOST`             | Hostname of the PostgreSQL database.             | `localhost`                              |
| `DB_PORT`             | Port of the PostgreSQL database.                 | `5432`                                   |
| `DB_USER`             | Username for the database.                       | `postgres`                               |
| `DB_PASSWORD`         | Password for the database.                       | `password`                               |
| `DB_NAME`             | Name of the database.                            | `irrigation_db`                          |
| `DB_SSLMODE`          | SSL mode for the database connection.            | `disable`                                |
| `SCHEDULE_TIME`       | Time to run the irrigation schedule (HH:MM).     | `07:00`                                  |
| `SCHEDULE_DURATION`   | Duration for the irrigation in minutes.          | `30`                                     |

## MQTT Topics

The system uses a device-specific topic structure. Replace `<deviceID>` with the actual ID of your sprinkler (e.g., `sprinkler_01`).

### Control Topics

-   `<deviceID>/control/valve/position`: Publish a float value (e.g., `90.0`) to set the valve position.
-   `<deviceID>/control/sprinkler/position`: Publish a float value (e.g., `-45.5`) to set the sprinkler's rotational position.

### Status Topics (Read-only)

The application subscribes to these topics to get real-time status from each device.

-   `<deviceID>/status/sprinkler/position`
-   `<deviceID>/status/valve/position`
-   `<deviceID>/status/sprinkler/calib_complete`
-   `<deviceID>/status/valve/calib_complete`
-   `<deviceID>/status/valve/is_at_target`
-   `<deviceID>/status/task/current_index`
-   `<deviceID>/status/task/current_count`
-   `<deviceID>/status/task/array`

## Database

The application uses a **PostgreSQL** database to store irrigation history. The database schema is automatically migrated on application startup.

## Project Structure

```
.
├── cmd/
│   └── irrigation/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/            # Configuration loading and structs
│   ├── models/             # Database and data models
│   ├── mqtt/               # MQTT client implementation
│   └── scheduler/          # Scheduling logic
├── .env.example           # Example environment file
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
