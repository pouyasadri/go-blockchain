#!/bin/bash
set -e

echo "=== Starting AI Blockchain Multi-Node P2P Cluster Simulation ==="

# Check docker compose
if ! command -v docker &> /dev/null; then
    echo "Docker is not installed or not in PATH."
    exit 1
fi

echo "1. Building Docker image and starting cluster..."
docker compose up --build -d

echo "2. Waiting for P2P mesh network stabilization (10 seconds)..."
sleep 10

echo "3. Cluster Status:"
docker compose ps

echo "4. Testing Web Dashboard availability on http://localhost:8080..."
if command -v curl &> /dev/null; then
    curl -I http://localhost:8080 || echo "Dashboard starting..."
fi

echo "=== Simulation environment is up! ==="
echo "To view live logs: docker compose logs -f"
echo "To stop cluster:  docker compose down -v"
