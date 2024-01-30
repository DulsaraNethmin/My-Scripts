#!/bin/bash

# Check if Docker is installed
if ! [ -x "$(command -v docker)" ]; then
  echo "Error: Docker is not installed." >&2
  exit 1
fi

# Check if Docker Compose is installed
if ! [ -x "$(command -v docker-compose)" ]; then
  echo "Error: Docker Compose is not installed." >&2
  exit 1
fi

# Navigate to the directory of your docker-compose.yml file
# Replace '/path/to/your/docker-compose-file' with the actual path
cd ./docker-compose.yml

# Run Docker Compose to build and start the containers
docker-compose up --build -d

echo "Docker containers are up and running."
