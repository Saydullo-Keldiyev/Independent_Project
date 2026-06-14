#!/bin/bash
# Start all services in development mode

set -e

echo "Starting infrastructure (postgres, redis, kafka)..."
docker-compose -f deployments/docker-compose.yml up -d postgres redis zookeeper kafka

echo "Waiting for services to be ready..."
sleep 5

echo "Running migrations..."
bash scripts/migrate.sh

echo "Starting services..."
go run services/user-service/cmd/main.go &
go run services/auction-service/cmd/main.go &
go run services/bid-service/cmd/main.go &
go run services/notification-service/cmd/main.go &
go run api-gateway/cmd/main.go &

echo "All services started. API Gateway: http://localhost:8080"
wait
