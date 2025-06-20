#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Check the environment and set variables accordingly
if [ "$APP_ENV" = "local" ]; then
    IMAGE_NAME="auto-irrigation-system-local"
    DOCKER_COMPOSE_FILE="docker-compose-local.yml"
elif [ "$APP_ENV" = "production" ]; then
    IMAGE_NAME="auto-irrigation-system-prod"
    DOCKER_COMPOSE_FILE="docker-compose-prod.yml"
else
    echo "Unknown environment: $APP_ENV"
    exit 1
fi

docker compose --env-file $SCRIPT_DIR/../.env.$APP_ENV -f $SCRIPT_DIR/../docker-compose/${DOCKER_COMPOSE_FILE} -p auto-irrigation-system up -d --build