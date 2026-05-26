#!/bin/bash

# Find script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "Stopping any process running on port 8080 (LISTEN)..."
lsof -n -i :8080 | grep LISTEN | awk '{print $2}' | xargs kill -9 2>/dev/null || true

echo "Waiting for port 8080 to be freed..."
sleep 2

echo "Starting Spring Boot server in background..."
cd "$SCRIPT_DIR/pathdb-server"
mvn spring-boot:run -Dspring-boot.run.jvmArguments="-Xmx6g -Xms6g" > "$SCRIPT_DIR/server_last_run.log" 2>&1 &

echo "Server restart initiated."
